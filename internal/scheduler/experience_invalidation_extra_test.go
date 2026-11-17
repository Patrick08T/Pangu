package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestExperienceInvalidationSourceWithdrawalAndOwnerRevocation(t *testing.T) {
	for _, kind := range []string{"source_deleted", "owner_disabled", "serving_safety"} {
		t.Run(kind, func(t *testing.T) {
			f := newExperienceInvalidationMemoryFixture(t)
			if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); err != nil {
				t.Fatal("healthy gate", err)
			}
			switch kind {
			case "source_deleted":
				rebuildExec(t, `DELETE FROM pangu.research_reports WHERE id=$1`, f.source)
			case "owner_disabled":
				rebuildExec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.actor)
			case "serving_safety":
				rebuildExec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,2,'paused','{"mode":"paused","block_frozen":true}',$2)`, f.version, f.actor)
			}
			if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, experience.ErrInvalidated) {
				t.Fatalf("unsafe gate accepted: %v", err)
			}
			assertExperienceInvalidation(t, f, "invalidated")
		})
	}
}

func TestExperienceInvalidationLegacyConsumedCheckpointFailsClosed(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	approveExperienceFixture(t, f, true)
	rebuildExec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'','','test','unused',true)`, f.run, f.project)
	out, _ := json.Marshal(workflow.NodeOutput{Text: "historical unsealed output", RunID: uuid.NewString()})
	rebuildExec(t, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output) VALUES($1,$2,'legacy',$3)`, uuid.NewString(), f.run, out)
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("legacy checkpoint treated as no dependency: %v", err)
	}
	assertExperienceInvalidation(t, f, "consumed")
}

func TestExperienceInvalidationScansBeyondFirstHundred(t *testing.T) {
	f := newExperienceInvalidationFixture(t, func(f *experienceInvalidationFixture, _ *workflow.Definition) { f.legacy = true })
	// Independent persisted node/generation identities deliberately have no accepted
	// definition/usage, representing old unproven gates rather than forged evidence.
	for i := 0; i < 104; i++ {
		id := uuid.NewString()
		rebuildExec(t, `INSERT INTO pangu.approvals(id,namespace,project_id,request_id,run_id,node_id,input_hash,subject_hash,subject,head_commit,round,approval_generation)
   SELECT $1,namespace,project_id,request_id,run_id,$2,input_hash,subject_hash,jsonb_set(subject,'{node_input,node,id}',to_jsonb($2::text)),head_commit,round,approval_generation FROM pangu.approvals WHERE id=$3`, id, fmt.Sprintf("legacy-%d", i), f.gate.ID)
	}
	if err := reconcileExperienceApprovals(context.Background(), f.ns); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var count int
	err = conn.QueryRow(context.Background(), `SELECT count(*) FROM pangu.approvals WHERE namespace=$1 AND invalidated_at IS NOT NULL`, f.ns).Scan(&count)
	if err != nil || count != 105 {
		t.Fatalf("incomplete paginated scan: %d %v", count, err)
	}
	if err = reconcileExperienceApprovals(context.Background(), f.ns); err != nil {
		t.Fatal(err)
	}
	intents, err := approval.RebuildIntents(context.Background(), f.ns, f.run)
	if err != nil || len(intents) != 105 {
		t.Fatalf("reconciliation duplicated/lost generations: %d %v", len(intents), err)
	}
}
