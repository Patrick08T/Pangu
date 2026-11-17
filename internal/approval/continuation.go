package approval

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func Continue(ctx context.Context, namespace, id string, fork *workflow.Fork) (*submission.Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return nil, err
	}
	gate, err := currentGate(ctx, tx, namespace, id)
	if err != nil {
		return nil, err
	}
	var existing string
	err = tx.QueryRow(ctx, "SELECT request_id::text FROM pangu.plan_continuations WHERE approval_id=$1", id).Scan(&existing)
	if err == nil {
		return submission.Get(ctx, namespace, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if gate.Status != "approved" || gate.Execution == nil || len(gate.Execution.Changes) == 0 || fork == nil || fork.Commit != gate.HeadCommit {
		return nil, ErrConflict
	}
	if err = continuationAuthority(ctx, tx, namespace, gate); err != nil {
		return nil, err
	}
	if err = checkHead(ctx, tx, gate); err != nil {
		return nil, err
	}
	if err = validateContinuationFork(ctx, tx, gate, fork); err != nil {
		return nil, err
	}
	version, err := catalog.PublishExecutionTx(ctx, tx, namespace, gate.Execution)
	if err != nil {
		return nil, err
	}
	d := gate.Execution.Workflow
	d.VersionID = version.ID
	item, err := submission.PrepareTx(ctx, tx, namespace, "plan", "plan:"+id, submission.Snapshot{Workflow: d, CreatedBy: gate.OwnerID, SourceRunID: gate.RunID, Fork: fork, ApprovalOrigin: id})
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO pangu.plan_continuations(approval_id,request_id,version_id) VALUES($1,$2,$3)", id, item.ID, version.ID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.approvals SET status='consumed',consumed_at=now() WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func continuationAuthority(ctx context.Context, tx pgx.Tx, namespace string, gate *Gate) error {
	if err := lockActiveRun(ctx, tx, gate.RunID); err != nil {
		return err
	}
	actor := identity.Actor{ID: gate.ActorID, Namespace: namespace}
	if err := identity.Authorize(ctx, actor, gate.ProjectID, "manage_run", gate.OwnerID); err != nil {
		return err
	}
	if err := identity.Authorize(ctx, actor, "", "manage_catalog", ""); err != nil {
		return err
	}
	allowed, err := identity.ScheduleOwner(ctx, tx, gate.OwnerID, gate.ProjectID)
	if err != nil {
		return err
	}
	if !allowed {
		return identity.ErrForbidden
	}
	return nil
}

func validateContinuationFork(ctx context.Context, tx pgx.Tx, gate *Gate, fork *workflow.Fork) error {
	before, err := workflow.BeforeGate(gate.Execution.Workflow, gate.NodeID)
	if err != nil {
		return err
	}
	if len(before) != len(fork.Reused) {
		return ErrConflict
	}
	return validateReusedOutputs(ctx, tx, gate.RunID, before, fork)
}

func validateReusedOutputs(ctx context.Context, tx pgx.Tx, run string, before map[string]bool, fork *workflow.Fork) error {
	for node, out := range fork.Reused {
		if !before[node] || out.Skipped || fork.Checkpoints[node] == "" {
			return ErrConflict
		}
		var raw []byte
		query := `SELECT output FROM pangu.workspace_checkpoints WHERE run_id=$1 AND node_run_id=$2 AND commit_id=$3`
		if fork.ReportNodes[node] {
			query = `SELECT output FROM pangu.research_reports WHERE run_id=$1 AND node_run_id=$2 AND $3::text<>''`
			var body []byte
			if err := tx.QueryRow(ctx, "SELECT body FROM pangu.research_reports WHERE run_id=$1 AND node_run_id=$2", run, out.RunID).Scan(&body); err != nil {
				return err
			}
			if fork.ReportHashes[node] != workflow.ReportHash(string(body)) {
				return ErrConflict
			}
		}
		if err := tx.QueryRow(ctx, query, run, out.RunID, fork.Checkpoints[node]).Scan(&raw); err != nil {
			return err
		}
		var stored workflow.NodeOutput
		if err := json.Unmarshal(raw, &stored); err != nil {
			return err
		}
		a, _ := json.Marshal(stored)
		b, _ := json.Marshal(out)
		if string(a) != string(b) {
			return ErrConflict
		}
	}
	return nil
}

func inheritedGate(ctx context.Context, tx pgx.Tx, namespace, run string, raw json.RawMessage, input workflow.NodeInput, head string) (*Gate, error) {
	var snapshot submission.Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.ApprovalOrigin == "" {
		return nil, nil
	}
	if input.ReviewPurpose == "evidence" {
		return nil, nil
	}
	gate, err := scan(tx.QueryRow(ctx, "SELECT "+columns+` FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id
 JOIN pangu.plan_continuations c ON c.approval_id=a.id JOIN pangu.submissions child ON child.id=c.request_id
 WHERE a.namespace=$1 AND a.id=$2 AND child.run_id=$3 AND c.version_id=$4`, namespace, snapshot.ApprovalOrigin, run, snapshot.Workflow.VersionID))
	if err != nil {
		return nil, err
	}
	if gate.NodeID != input.Node.ID {
		return nil, nil
	}
	if err = checkGateGeneration(ctx, tx, namespace, gate); err != nil {
		return nil, err
	}
	if gate.Status != "consumed" || gate.Execution == nil || gate.HeadCommit != head || gate.ProjectID != input.ProjectID || snapshot.Fork == nil {
		return nil, ErrConflict
	}
	var node workflow.Node
	for _, n := range snapshot.Workflow.Nodes {
		if n.ID == gate.NodeID {
			node = n
		}
	}
	expected, err := snapshot.Workflow.BindInput(snapshot.Workflow.NodeInput(node, snapshot.Fork.Reused))
	if err != nil {
		return nil, err
	}
	expected, err = reportInput(ctx, gate.RunID, expected)
	if err != nil {
		return nil, err
	}
	expected.Experiences, err = inheritedExperiences(ctx, tx, namespace, run, gate, input)
	if err != nil {
		return nil, err
	}
	if !sameInheritedInput(expected, input) {
		return nil, ErrConflict
	}
	return gate, nil
}

func sameInheritedInput(a, b workflow.NodeInput) bool {
	normalize := func(in workflow.NodeInput) []byte {
		in.ApprovalRound, in.ApprovalID, in.RequestID = 0, "", ""
		in.ApprovalGeneration = normalizedGeneration(in.ApprovalGeneration)
		parents := map[string]workflow.NodeOutput{}
		for id, out := range in.Parents {
			out.RunID = ""
			out.ReportIDs = nil
			parents[id] = out
		}
		in.Parents = parents
		raw, _ := json.Marshal(in)
		return raw
	}
	return string(normalize(a)) == string(normalize(b))
}
