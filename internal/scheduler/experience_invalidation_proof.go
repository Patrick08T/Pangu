package scheduler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func auditExperienceGate(ctx context.Context, namespace string, gate *approval.Gate) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	var request string
	err = conn.QueryRow(ctx, `SELECT request_id::text FROM pangu.approvals WHERE namespace=$1 AND id=$2`, namespace, gate.ID).Scan(&request)
	conn.Close(context.Background())
	if err != nil {
		return err
	}
	r, err := loadNodeExperience(ctx, namespace, request, gate.RunID, gate.ProjectID)
	if err != nil {
		return err
	}
	if r.request.RunID != gate.RunID || r.request.CreatedBy != gate.OwnerID || r.request.Snapshot.CreatedBy != r.request.CreatedBy {
		return experience.ErrInvalidated
	}
	if r.request.CreatedBy != "" {
		actor := identity.Actor{ID: r.request.CreatedBy, Namespace: namespace}
		if err = identity.Authorize(ctx, actor, gate.ProjectID, "run", r.request.CreatedBy); err != nil {
			return err
		}
	}
	var subject struct {
		Input workflow.NodeInput `json:"node_input"`
	}
	if len(gate.Subject) == 0 || json.Unmarshal(gate.Subject, &subject) != nil {
		return experience.ErrInvalidated
	}
	input := subject.Input
	if input.Node.ID != gate.NodeID || input.ProjectID != gate.ProjectID || input.Input != r.request.Snapshot.Workflow.Input {
		return experience.ErrInvalidated
	}
	if input.ApprovalGeneration == 0 {
		input.ApprovalGeneration = 1
	}
	if input.ApprovalGeneration != gate.ApprovalGeneration || input.ApprovalRound != gate.Round {
		return experience.ErrInvalidated
	}
	if gate.Purpose == "evidence" {
		err = r.auditEvidenceGate(ctx, input)
	} else {
		err = r.auditSelectedInput(ctx, input)
	}
	if err != nil {
		return err
	}
	if err = r.auditFrozenCheckpoints(ctx); err != nil {
		return err
	}
	return r.auditConsumedArtifacts(ctx, gate)
}

func (r *nodeExperience) auditSelectedInput(ctx context.Context, input workflow.NodeInput) error {
	if err := r.checkDefinition(input); err != nil {
		return err
	}
	if err := r.checkParents(ctx, input); err != nil {
		return err
	}
	role, err := r.roleID(ctx, input.Node.AgentVersion)
	if err != nil {
		return err
	}
	selection, err := experienceSelection(r.request.ID, input, role, r.request.Operation)
	if err != nil {
		return experience.ErrInvalidated
	}
	if err = r.requireFrozenUsage(ctx, selection); err != nil {
		return err
	}
	usage, versions, err := r.store.Select(ctx, r.scope, selection)
	if err != nil {
		return err
	}
	expected, err := experiencePayload(usage, versions)
	if err != nil {
		return err
	}
	before, err := workflow.EffectiveInputHash(input)
	if err != nil {
		return experience.ErrInvalidated
	}
	input.Experiences = expected
	after, err := workflow.EffectiveInputHash(input)
	if err != nil {
		return experience.ErrInvalidated
	}
	if before != after {
		return experience.ErrInvalidated
	}
	return nil
}

func (r *nodeExperience) requireFrozenUsage(ctx context.Context, s experience.SelectionInput) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var exists bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2
 AND usage->'input'->>'run_id'=$3 AND usage->'input'->>'node_id'=$4
 AND usage->'input'->>'bound_input_hash'=$5 AND usage->'input'->>'round'=$6
 AND usage->'input'->>'generation'=$7 AND usage->'input'->>'approval_revision'=$8)`,
		r.namespace, r.scope.ProjectID, s.RunID, s.NodeID, s.BoundInputHash, jsonNumber(s.Round), jsonNumber(s.Generation), jsonNumber(s.ApprovalRevision)).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return experience.ErrInvalidated
	}
	return nil
}

func jsonNumber(value any) string { data, _ := json.Marshal(value); return string(data) }

func (r *nodeExperience) auditEvidenceGate(ctx context.Context, input workflow.NodeInput) error {
	if input.ReviewPurpose != "evidence" || input.Node.Approval == nil || !input.Node.ReadOnly || input.Node.Executor != "codex" || !strings.HasPrefix(input.Node.ID, "evidence-") {
		return experience.ErrInvalidated
	}
	source := input.Node.Approval.SourceNode
	accepted := false
	for _, node := range r.request.Snapshot.Workflow.Nodes {
		if node.ID == source && node.ReadOnly && node.Approval == nil {
			accepted = true
		}
	}
	if !accepted {
		return experience.ErrInvalidated
	}
	artifact, err := r.store.ReadArtifact(ctx, r.scope, strings.TrimPrefix(input.Node.ID, "evidence-"))
	if err != nil {
		return err
	}
	if artifact.RunID != r.runID || artifact.NodeID != source || artifact.ProofVersion != 1 || len(artifact.ExecutionInputHash) != 64 {
		return experience.ErrInvalidated
	}
	return r.store.ValidateReferences(ctx, r.scope, memoryReferences(input))
}

func (r *nodeExperience) auditFrozenCheckpoints(ctx context.Context) error {
	fork := r.request.Snapshot.Fork
	if fork == nil {
		return nil
	}
	for node := range fork.Checkpoints {
		if _, ok := fork.Reused[node]; !ok {
			return experience.ErrInvalidated
		}
	}
	return r.checkParents(ctx, workflow.NodeInput{Parents: fork.Reused})
}

func (r *nodeExperience) auditConsumedArtifacts(ctx context.Context, gate *approval.Gate) error {
	if gate.ConsumedAt == nil && gate.Status != "consumed" {
		return nil
	}
	ids, err := r.approvalArtifactIDs(ctx, gate)
	if err != nil {
		return err
	}
	for _, id := range ids {
		artifact, err := r.store.ReadArtifact(ctx, r.scope, id)
		if err != nil {
			return err
		}
		if artifact.ProofVersion != 1 || len(artifact.ExecutionInputHash) != 64 {
			return experience.ErrInvalidated
		}
	}
	return r.auditPublishedOutputs(ctx)
}

func (r *nodeExperience) approvalArtifactIDs(ctx context.Context, gate *approval.Gate) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2
 AND artifact->>'run_id'=$3 AND artifact->>'node_id'=$4 AND artifact->>'generation'=$5 ORDER BY id`,
		r.namespace, r.scope.ProjectID, r.runID, gate.NodeID, jsonNumber(gate.ApprovalGeneration))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
