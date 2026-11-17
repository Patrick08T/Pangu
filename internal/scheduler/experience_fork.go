package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

// validateForkExperience runs after forkSnapshot, before accepting any fork effects.
// It returns no source report bodies and never treats success as proof of lineage.
func validateForkExperience(ctx context.Context, namespace string, previous *submission.Record, snapshot *submission.Snapshot) error {
	if previous == nil || snapshot == nil || snapshot.Fork == nil {
		return experience.ErrInvalidated
	}
	source, err := submission.ForRun(ctx, namespace, snapshot.SourceRunID)
	if err != nil {
		return err
	}
	if source.ID != previous.ID || source.RunID != previous.RunID {
		return experience.ErrInvalidated
	}
	reader, err := forkExperienceReader(ctx, namespace, source, nil)
	if err != nil {
		return err
	}
	_, err = reader.validateForkSource(ctx, source, snapshot)
	return err
}

func forkExperienceReader(ctx context.Context, namespace string, source *submission.Record, target *nodeExperience) (*nodeExperience, error) {
	actor := identity.Current(ctx)
	if actor.ID != "" {
		return &nodeExperience{namespace: namespace, runID: source.RunID, request: source, scope: experience.Scope{Namespace: namespace, ProjectID: source.ProjectID, ActorID: actor.ID}, store: learning.NewStore()}, nil
	}
	if target != nil {
		return target, nil
	}
	// Anonymous reads use only the exact server-bound request and its empty watermark.
	return loadNodeExperience(ctx, namespace, source.ID, source.RunID, source.ProjectID)
}

func (r *nodeExperience) forkContext(ctx context.Context) (*submission.Record, []string, error) {
	current, err := submission.ForRun(ctx, r.namespace, r.runID)
	if err != nil {
		return nil, nil, err
	}
	if current.ID != r.request.ID || current.ProjectID != r.scope.ProjectID || !reflect.DeepEqual(current.Snapshot, r.request.Snapshot) {
		return nil, nil, experience.ErrInvalidated
	}
	if current.Snapshot.Fork == nil {
		return nil, nil, nil
	}
	if current.Operation != "fork" || current.Snapshot.SourceRunID == r.runID {
		return nil, nil, experience.ErrInvalidated
	}
	source, err := submission.ForRun(ctx, r.namespace, current.Snapshot.SourceRunID)
	if err != nil {
		return nil, nil, err
	}
	reader, err := forkExperienceReader(ctx, r.namespace, source, r)
	if err != nil {
		return nil, nil, err
	}
	order, err := reader.validateForkSource(ctx, source, &current.Snapshot)
	return source, order, err
}

// ValidateFork must precede workspace creation; ImportFork repeats it before import.
func (r *nodeExperience) ValidateFork(ctx context.Context) error {
	_, _, err := r.forkContext(ctx)
	return err
}

func (r *nodeExperience) validateForkSource(ctx context.Context, source *submission.Record, snapshot *submission.Snapshot) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fork := snapshot.Fork
	if fork == nil || source.RunID != snapshot.SourceRunID || source.ProjectID != snapshot.Workflow.ProjectID || source.ProjectID != r.scope.ProjectID {
		return nil, experience.ErrInvalidated
	}
	order, err := workflow.Prefix(snapshot.Workflow, source.Snapshot.Workflow, fork.CheckpointNode)
	if err != nil {
		return nil, err
	}
	if len(order) != len(fork.Reused) || len(order) != len(fork.Checkpoints) {
		return nil, experience.ErrInvalidated
	}
	ws, err := workspace.RerunSource(ctx, source.RunID, source.ProjectID)
	if err != nil {
		return nil, err
	}
	origin := ws.BaseCommit
	if source.Snapshot.Fork != nil {
		origin = source.Snapshot.Fork.OriginCommit
	}
	if fork.OriginCommit != origin {
		return nil, experience.ErrInvalidated
	}
	commit := origin
	for _, node := range order {
		out, ok := fork.Reused[node]
		if !ok {
			return nil, experience.ErrInvalidated
		}
		if _, err = r.forkOutputProof(ctx, source.RunID, node, out); err != nil {
			return nil, err
		}
		next, err := readForkCheckpoint(ctx, source.RunID, node, commit, fork)
		if err != nil {
			return nil, err
		}
		commit = next
	}
	if commit != fork.Commit {
		return nil, experience.ErrInvalidated
	}
	for node := range fork.ReportNodes {
		if _, ok := fork.Reused[node]; !ok {
			return nil, experience.ErrInvalidated
		}
	}
	for node := range fork.ReportHashes {
		if _, ok := fork.Reused[node]; !ok {
			return nil, experience.ErrInvalidated
		}
	}
	return order, nil
}

func readForkCheckpoint(ctx context.Context, run, node, previous string, fork *workflow.Fork) (string, error) {
	value := fork.Reused[node]
	var actual workflow.NodeOutput
	var commit string
	var err error
	if fork.ReportNodes[node] {
		actual, err = research.Checkpoint(ctx, run, value.RunID)
		commit = previous
		if err == nil && (len(actual.Reports) != 1 || workflow.ReportHash(actual.Reports[0].Markdown) != fork.ReportHashes[node]) {
			return "", experience.ErrInvalidated
		}
	} else {
		commit, actual, err = workspace.Checkpoint(ctx, run, value.RunID)
	}
	if err != nil {
		return "", err
	}
	if commit != fork.Checkpoints[node] || !reflect.DeepEqual(actual.Lineage, value.Lineage) {
		return "", experience.ErrInvalidated
	}
	before, err := workflow.OutputFingerprint(actual)
	if err != nil {
		return "", err
	}
	after, err := workflow.OutputFingerprint(value)
	if err != nil {
		return "", err
	}
	if before != after {
		return "", experience.ErrInvalidated
	}
	return commit, checkForkReport(ctx, run, value)
}

func (r *nodeExperience) forkOutputProof(ctx context.Context, run, node string, out workflow.NodeOutput) (experience.Artifact, error) {
	ref, err := outputReference(out)
	if err != nil {
		return experience.Artifact{}, err
	}
	a, err := r.store.ReadArtifact(ctx, r.scope, ref.ID)
	if err != nil {
		return a, err
	}
	if a.ID != out.RunID || a.RunID != run || a.NodeID != node || a.ProjectID != r.scope.ProjectID || a.Generation < 1 || a.ProofVersion != 1 ||
		(a.Kind != "node_output" && a.Kind != "checkpoint_import") || a.Hash != ref.Hash || a.InputHash != out.Lineage.InputHash ||
		len(a.ExecutionInputHash) != 64 || a.ExecutionInputHash != out.Lineage.ExecutionInputHash {
		return a, experience.ErrInvalidated
	}
	return a, nil
}

// ImportFork signs only the final remapped checkpoint/report and preserves source
// memory dependencies through its source parent. It never selects new experiences.
func (r *nodeExperience) ImportFork(ctx context.Context) (map[string]workflow.NodeOutput, error) {
	source, order, err := r.forkContext(ctx)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return map[string]workflow.NodeOutput{}, nil
	}
	imported := map[string]workflow.NodeOutput{}
	observe := func(ctx context.Context, node, commit string, out *workflow.NodeOutput, cached bool) error {
		if _, _, err := r.forkContext(ctx); err != nil {
			return err
		}
		artifact, err := r.forkImportArtifact(ctx, source, node, commit, *out, imported)
		if err != nil {
			return err
		}
		if cached {
			err = r.validateImported(ctx, *out, artifact)
		} else {
			err = r.store.SaveArtifact(ctx, r.scope, artifact)
			if err == nil {
				out.Lineage = &workflow.OutputLineage{Version: 1, ArtifactID: artifact.ID, InputHash: artifact.InputHash, OutputHash: artifact.Hash, ExecutionInputHash: artifact.ExecutionInputHash}
			}
		}
		if err == nil {
			imported[node] = *out
		}
		return err
	}
	return workspace.ImportCheckpointsObserved(ctx, r.runID, r.request.Snapshot.Fork, order, observe)
}

// ValidateReused is mounted before RunAudited: reused nodes skip normal execution
// and Observe, so accepting arbitrary run.Reused would bypass the entire contract.
func (r *nodeExperience) ValidateReused(ctx context.Context, reused map[string]workflow.NodeOutput) error {
	source, order, err := r.forkContext(ctx)
	if err != nil {
		return err
	}
	if len(reused) != len(order) {
		return experience.ErrInvalidated
	}
	if source == nil {
		return nil
	}
	imported := map[string]workflow.NodeOutput{}
	for _, node := range order {
		out, ok := reused[node]
		if !ok {
			return experience.ErrInvalidated
		}
		commit, actual, err := workspace.Checkpoint(ctx, r.runID, out.RunID)
		if err != nil {
			return err
		}
		if commit != r.request.Snapshot.Fork.Checkpoints[node] || !reflect.DeepEqual(actual.Lineage, out.Lineage) {
			return experience.ErrInvalidated
		}
		a, err := r.forkImportArtifact(ctx, source, node, commit, out, imported)
		if err != nil {
			return err
		}
		if err = r.validateImported(ctx, actual, a); err != nil {
			return err
		}
		if err = r.validateImported(ctx, out, a); err != nil {
			return err
		}
		imported[node] = out
	}
	return nil
}

func (r *nodeExperience) forkImportArtifact(ctx context.Context, source *submission.Record, node, commit string, out workflow.NodeOutput, imported map[string]workflow.NodeOutput) (experience.Artifact, error) {
	a := experience.Artifact{}
	id, err := uuid.Parse(r.runID)
	if err != nil {
		return a, err
	}
	if out.RunID != uuid.NewSHA1(id, []byte(node)).String() || commit != r.request.Snapshot.Fork.Checkpoints[node] {
		return a, experience.ErrInvalidated
	}
	input, err := r.forkLogicalInput(node, imported)
	if err != nil {
		return a, err
	}
	if err = r.checkParents(ctx, input); err != nil {
		return a, err
	}
	sourceOutput, ok := r.request.Snapshot.Fork.Reused[node]
	if !ok {
		return a, experience.ErrInvalidated
	}
	original, err := r.forkOutputProof(ctx, source.RunID, node, sourceOutput)
	if err != nil {
		return a, err
	}
	a = experience.Artifact{ID: out.RunID, ProjectID: r.scope.ProjectID, RunID: r.runID, NodeID: node, Kind: "checkpoint_import", Generation: 1, ProofVersion: 1, Parents: []experience.ArtifactReference{{ID: original.ID, Hash: original.Hash}}}
	a.InputHash, err = workflow.EffectiveInputHash(input)
	if err != nil {
		return a, err
	}
	a.Hash, err = workflow.OutputFingerprint(out)
	if err != nil {
		return a, err
	}
	ids := make([]string, 0, len(input.Parents))
	for id := range input.Parents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		ref, err := outputReference(input.Parents[id])
		if err != nil {
			return a, err
		}
		a.Parents = append(a.Parents, ref)
	}
	a.ExecutionInputHash, err = forkImportExecutionHash(r, source, commit, a)
	return a, err
}

func (r *nodeExperience) forkLogicalInput(node string, imported map[string]workflow.NodeOutput) (workflow.NodeInput, error) {
	d := r.request.Snapshot.Workflow
	for _, n := range d.Nodes {
		if n.ID != node {
			continue
		}
		for _, id := range n.DependsOn {
			if _, ok := imported[id]; !ok {
				return workflow.NodeInput{}, experience.ErrInvalidated
			}
		}
		input := d.NodeInput(n, imported)
		input.RequestID = r.request.ID
		return d.BindInput(input)
	}
	return workflow.NodeInput{}, experience.ErrInvalidated
}

func forkImportExecutionHash(r *nodeExperience, source *submission.Record, commit string, artifact experience.Artifact) (string, error) {
	// Canonical measured import context, not a claimed fresh model execution.
	value := struct {
		Kind            string              `json:"kind"`
		Namespace       string              `json:"namespace"`
		RequestID       string              `json:"request_id"`
		SourceRequestID string              `json:"source_request_id"`
		SourceRunID     string              `json:"source_run_id"`
		Commit          string              `json:"commit"`
		Artifact        experience.Artifact `json:"artifact"`
	}{"checkpoint_import", r.namespace, r.request.ID, source.ID, source.RunID, commit, artifact}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (r *nodeExperience) validateImported(ctx context.Context, out workflow.NodeOutput, expected experience.Artifact) error {
	actual, err := r.forkOutputProof(ctx, r.runID, expected.NodeID, out)
	if err != nil {
		return err
	}
	// Compare every authority field, not merely the output fingerprint.
	a, err := json.Marshal(actual)
	if err != nil {
		return err
	}
	b, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if string(a) != string(b) {
		return experience.ErrInvalidated
	}
	return checkForkReport(ctx, r.runID, out)
}
