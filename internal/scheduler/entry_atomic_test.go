package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func TestEntryAcceptanceRollbackIncludesFrozenExperience(t *testing.T) {
	f := newEntryFixture(t)
	d, err := catalog.Compile(f.ctx, f.actor.Namespace, f.version.ID, f.project, "task")
	if err != nil {
		t.Fatal(err)
	}
	request := entryRequest{namespace: f.actor.Namespace, project: f.project, entry: "workbench:main", input: "task", key: "rollback"}
	rollback := errors.New("acceptance transaction aborted")
	var id string
	err = entryTransaction(f.ctx, request, func(tx pgx.Tx) error {
		snapshot := submission.Snapshot{Purpose: "business", Workflow: *d, EntryKey: request.entry, EntryRevision: 1}
		item, err := submission.PrepareTx(f.ctx, tx, request.namespace, "entry", request.key, snapshot)
		if err != nil {
			return err
		}
		id = item.ID
		var frozen bool
		if err = tx.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pangu.experience_watermarks WHERE namespace=$1 AND run_id=$2)`, request.namespace, id).Scan(&frozen); err != nil {
			return err
		}
		if !frozen {
			t.Error("freeze missing before transaction commit")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var n int
	if err = entryQuery(t, `SELECT count(*) FROM pangu.experience_watermarks WHERE namespace=$1 AND run_id=$2`, f.actor.Namespace, id).Scan(&n); err != nil || n != 0 {
		t.Fatal("freeze survived failed acceptance", n, err)
	}
	if n = entrySubmissionCount(t, f); n != 0 {
		t.Fatal("acceptance survived rollback", n)
	}
}

func TestEntryCASIncludesVersionStableAndPolicyRevision(t *testing.T) {
	for _, kind := range []string{"version", "stable", "policy"} {
		t.Run(kind, func(t *testing.T) {
			f := newEntryFixture(t)
			replacement := f.automatic(t, f.project, "stable")
			check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
				switch kind {
				case "version":
					entryExec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$2 WHERE namespace=$1`, f.actor.Namespace, replacement)
				case "stable":
					entryExec(t, `UPDATE pangu.evolution_bindings SET stable_version=$2 WHERE namespace=$1`, f.actor.Namespace, replacement)
				case "policy":
					f.policy.Revision++
					f.savePolicy(t)
				}
				return &workflow.Preflight{Ready: true}, nil
			}
			_, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "full-cas", entryHooks{check, entryPending})
			if !errors.Is(err, submission.ErrConflict) {
				t.Fatal("acceptance ignored changed identity", err)
			}
			if n := entrySubmissionCount(t, f); n != 0 {
				t.Fatal("stale selection persisted", n)
			}
		})
	}
}
