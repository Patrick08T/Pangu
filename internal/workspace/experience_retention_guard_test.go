//go:build integration

package workspace

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestExperienceRetentionPurgeRequiresConfirmedStop(t *testing.T) {
	for _, mode := range []string{"unconfirmed", "missing", "active"} {
		t.Run(mode, func(t *testing.T) {
			tx, request, run := experienceRetentionTx(t)
			switch mode {
			case "unconfirmed":
				experienceRetentionExec(t, tx, `UPDATE pangu.evolution_case_runtime SET stop_confirmed=false WHERE request_id=$1`, request)
			case "missing":
				experienceRetentionExec(t, tx, `DELETE FROM pangu.evolution_case_runtime WHERE request_id=$1`, request)
			case "active":
				experienceRetentionExec(t, tx, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt) VALUES($1,$2,'answer',1)`, run, uuid.NewString())
			}
			experienceRetentionReject(t, tx, `SELECT pangu.purge_run_payloads($1)`, run)
			var present bool
			if err := tx.QueryRow(context.Background(), `SELECT plan IS NOT NULL FROM pangu.evolution_case_plans WHERE request_id=$1`, request).Scan(&present); err != nil {
				t.Fatal(err)
			}
			if !present {
				t.Fatal("unsafe purge partially erased the plan")
			}
		})
	}
}

func experienceRetentionReject(t *testing.T, tx pgx.Tx, query string, arg string) {
	t.Helper()
	child, err := tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer child.Rollback(context.Background())
	if _, err = child.Exec(context.Background(), query, arg); err == nil {
		t.Fatal("unsafe private-body mutation accepted")
	}
}

func TestExperienceRetentionPurgeCannotRestoreBodies(t *testing.T) {
	tx, request, run := experienceRetentionTx(t)
	experienceRetentionExec(t, tx, `SELECT pangu.purge_run_payloads($1)`, run)
	for _, query := range []string{
		`UPDATE pangu.evolution_case_plans SET plan='{"restored":true}',erased_at=NULL WHERE request_id=$1`,
		`UPDATE pangu.evolution_case_runtime SET outputs='{"restored":true}' WHERE request_id=$1`,
		`UPDATE pangu.evolution_node_runs SET output='{"restored":true}' WHERE request_id=$1`,
		`UPDATE pangu.evolution_case_runtime SET status='running',stop_confirmed=false,finished_at=NULL WHERE request_id=$1`,
	} {
		experienceRetentionReject(t, tx, query, request)
	}
}

func TestExperienceRetentionPurgeKeepsAllInternalPurposes(t *testing.T) {
	for _, purpose := range []string{"business", "learning", "evaluation"} {
		t.Run(purpose, func(t *testing.T) {
			tx, request, run := experienceRetentionTx(t)
			experienceRetentionExec(t, tx, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{purpose}',to_jsonb($2::text)) WHERE id=$1`, request, purpose)
			experienceRetentionExec(t, tx, `SELECT pangu.purge_run_payloads($1)`, run)
			var got string
			var minimal bool
			err := tx.QueryRow(context.Background(), `SELECT snapshot->>'purpose',
		 snapshot#>>'{workflow,version_id}'='audit-version' AND snapshot#>'{workflow,budget}' IS NOT NULL
		 AND NOT (snapshot->'fork' ? 'reused') AND NOT (snapshot->'workflow' ? 'nodes')
		 FROM pangu.submissions WHERE id=$1`, request).Scan(&got, &minimal)
			if err != nil {
				t.Fatal(err)
			}
			if got != purpose || !minimal {
				t.Fatalf("metadata=%s minimal=%v", got, minimal)
			}
		})
	}
}
