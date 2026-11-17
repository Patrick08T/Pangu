package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

type evolutionBusinessRun struct {
	RunID, RequestID, TaskID, OwnerID, EntryKey, InputHash, SourceBase, Operation string
	Snapshot                                                                      submission.Snapshot
	SnapshotBody, Summary, Feedback                                               []byte
	CreatedAt                                                                     time.Time
	Status                                                                        string
	StopConfirmed                                                                 bool
	Decision, FirstDecision                                                       string
	MemoryHash, MemoryEvidenceHash                                                string
}

func evolutionBusinessRunTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, id string) (evolutionBusinessRun, error) {
	out := evolutionBusinessRun{RunID: id}
	err := tx.QueryRow(ctx, `SELECT s.id::text,s.task_id::text,coalesce(s.created_by::text,''),s.operation,s.input_hash,s.snapshot,s.created_at,h.summary,w.base_commit,to_jsonb(f),f.decision,f.first_decision,
 coalesce((SELECT 'cron:'||sf.schedule_id::text FROM pangu.schedule_fires sf WHERE sf.request_id=s.id LIMIT 1),(SELECT 'webhook:'||g.hook_id::text FROM pangu.github_events g WHERE g.request_id=s.id LIMIT 1),s.snapshot->>'entry_key','')
 FROM pangu.submissions s JOIN pangu.run_history h ON h.run_id=s.run_id AND h.namespace=s.namespace JOIN pangu.workspace_runs w ON w.run_id=s.run_id AND w.project_id=s.project_id
 JOIN pangu.feedback f ON f.run_id=s.run_id AND f.namespace=s.namespace AND f.project_id=s.project_id AND f.task_id=s.task_id
 JOIN pangu.users u ON u.id=f.actor_id AND u.namespace=s.namespace AND NOT u.disabled
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.run_id=$3 AND s.purged_at IS NULL AND w.expired_at IS NULL AND w.purged_at IS NULL AND NOT f.payload_expired AND f.source='feishu_session'
 AND (u.is_admin OR EXISTS(SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=s.project_id AND m.enabled AND (m.role='manager' OR (m.role='operator' AND s.created_by=u.id))))
 AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts a WHERE a.run_id=s.run_id AND a.finished_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM pangu.evaluation_identity_allocations a WHERE a.namespace=s.namespace AND a.project_id=s.project_id AND a.partition IN ('holdout','protection') AND ((a.identity_kind='source' AND a.identity_key=s.run_id::text) OR (a.identity_kind='task' AND a.identity_key=s.task_id::text)))
 AND NOT EXISTS(SELECT 1 FROM pangu.evaluation_case_allocations a WHERE a.namespace=s.namespace AND a.project_id=s.project_id AND a.partition IN ('holdout','protection') AND a.input_hash=s.input_hash)
 FOR SHARE OF s,h,w,f,u`, s.Namespace, s.ProjectID, id).Scan(&out.RequestID, &out.TaskID, &out.OwnerID, &out.Operation, &out.InputHash, &out.SnapshotBody, &out.CreatedAt, &out.Summary, &out.SourceBase, &out.Feedback, &out.Decision, &out.FirstDecision, &out.EntryKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, evolution.ErrChecker
	}
	if err != nil {
		return out, err
	}
	var summary struct {
		ProjectID     string `json:"project_id"`
		Status        string `json:"status"`
		StopConfirmed bool   `json:"stop_confirmed"`
	}
	if json.Unmarshal(out.SnapshotBody, &out.Snapshot) != nil || json.Unmarshal(out.Summary, &summary) != nil {
		return out, evolution.ErrChecker
	}
	out.Status, out.StopConfirmed = summary.Status, summary.StopConfirmed
	if out.EntryKey == "" || !out.StopConfirmed || summary.ProjectID != s.ProjectID || out.Snapshot.Workflow.ProjectID != s.ProjectID || out.Snapshot.Workflow.Input == "" || evolutionTextHash(out.Snapshot.Workflow.Input) != out.InputHash || !slices.Contains([]string{"succeeded", "failed", "cancelled"}, out.Status) {
		return out, evolution.ErrChecker
	}
	if !evolutionBusinessOrigin(out) || !slices.Contains([]string{"submit", "cron", "github", "entry"}, out.Operation) {
		return out, evolution.ErrChecker
	}
	if err = evolutionCanonicalBusinessTx(ctx, tx, s, out.Snapshot); err != nil {
		return out, err
	}
	out.MemoryHash, out.MemoryEvidenceHash, err = evolutionBusinessMemoryTx(ctx, tx, s, out)
	return out, err
}

func evolutionCanonicalBusinessTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, snap submission.Snapshot) error {
	if snap.Workflow.VersionID == "" {
		return evolution.ErrChecker
	}
	scope := catalog.VersionScope{ProjectID: s.ProjectID, AllowObserving: true}
	ids := []string{snap.Workflow.VersionID}
	for _, n := range snap.Workflow.Nodes {
		if n.AgentVersion != "" {
			ids = append(ids, n.AgentVersion)
		}
		if n.ConnectionVersion != "" {
			ids = append(ids, n.ConnectionVersion)
		}
	}
	for _, id := range ids {
		if _, err := catalog.ReadVersionScopedTx(ctx, tx, s.Namespace, id, scope); err != nil {
			return err
		}
	}
	canonical, err := catalog.CompileScoped(ctx, s.Namespace, snap.Workflow.VersionID, scope, snap.Workflow.Input)
	if err != nil {
		return err
	}
	if catalog.CompiledHash(*canonical) != catalog.CompiledHash(snap.Workflow) {
		return evolution.ErrChecker
	}
	return nil
}

func evolutionComparable(a, b evolutionBusinessRun) bool {
	if a.InputHash != b.InputHash || a.SourceBase == "" || a.SourceBase != b.SourceBase || a.OwnerID != b.OwnerID || a.EntryKey != b.EntryKey || a.TaskID == b.TaskID || a.MemoryHash != b.MemoryHash {
		return false
	}
	left, right := a.Snapshot.Workflow, b.Snapshot.Workflow
	// The tested role instruction/version are the only admitted differences.
	left.Nodes = slices.Clone(left.Nodes)
	right.Nodes = slices.Clone(right.Nodes)
	left.VersionID, right.VersionID = "", ""
	for i := range left.Nodes {
		left.Nodes[i].Instruction = ""
		left.Nodes[i].AgentVersion = ""
	}
	for i := range right.Nodes {
		right.Nodes[i].Instruction = ""
		right.Nodes[i].AgentVersion = ""
	}
	return evolutionHash(left) == evolutionHash(right)
}

func evolutionBusinessScore(ctx context.Context, tx pgx.Tx, run evolutionBusinessRun, metric string) (float64, string, error) {
	accepted := run.Status == "succeeded" && run.Decision == "accepted" && run.FirstDecision == "accepted"
	if metric == "acceptance" {
		if accepted {
			return 1, "", nil
		}
		return 0, "", nil
	}
	// Lower cost/latency never counts as benefit when independent acceptance failed.
	if !accepted {
		return 0, "", evolution.ErrChecker
	}
	if metric == "cost" {
		r := evolution.MachineResult{RunID: run.RunID}
		if err := evolutionUsageTx(ctx, tx, &r); err != nil {
			return 0, "", err
		}
		if !r.CostMeasured {
			return 0, "", evolution.ErrChecker
		}
		return r.Cost, r.Currency, nil
	}
	if metric == "duration" {
		var measured bool
		var seconds float64
		err := tx.QueryRow(ctx, `SELECT count(*)>0 AND count(*)=count(finished_at),coalesce(extract(epoch FROM max(finished_at)-min(started_at)),0) FROM pangu.node_attempts WHERE run_id=$1`, run.RunID).Scan(&measured, &seconds)
		if err != nil {
			return 0, "", err
		}
		if !measured || seconds <= 0 {
			return 0, "", evolution.ErrChecker
		}
		return seconds, "", nil
	}
	return 0, "", evolution.ErrChecker
}
