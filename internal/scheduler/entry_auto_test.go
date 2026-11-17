package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

// Seed storage identity/lifecycle only: no fabricated model result, receipt, score or passed decision.
func (f entryFixture) automatic(t *testing.T, project, state string) string {
	t.Helper()
	return f.automaticFrom(t, project, state, "")
}

func (f entryFixture) automaticFrom(t *testing.T, project, state, previous string) string {
	t.Helper()
	pool, suite, experiment, id := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	hash := entryDigest(experiment)
	body, err := json.Marshal(f.version.Spec)
	if err != nil {
		t.Fatal(err)
	}
	entryExec(t, `INSERT INTO pangu.evaluation_holdout_pools(id,namespace,project_id,holdout_hash,rules_hash,max_candidates) VALUES($1,$2,$3,$4,$4,1)`, pool, f.actor.Namespace, project, hash)
	entryExec(t, `INSERT INTO pangu.evaluation_suites(id,namespace,project_id,pool_id,spec,spec_hash) VALUES($1,$2,$3,$4,'{}',$5)`, suite, f.actor.Namespace, project, pool, hash)
	entryExec(t, `INSERT INTO pangu.evaluation_experiments(id,namespace,project_id,suite_id,generation,spec,spec_hash) VALUES($1,$2,$3,$4,1,'{}',$5)`, experiment, f.actor.Namespace, project, suite, hash)
	entryExec(t, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec,source_kind,source_experiment,project_id,public_baseline,content_hash,project_previous)
 SELECT $1,object_id,(SELECT max(number)+1 FROM pangu.catalog_versions WHERE object_id=v.object_id),NULL,'{}','auto',$2,$3,$4,$5,nullif($6,'')::uuid FROM pangu.catalog_versions v WHERE id=$4`, id, experiment, project, f.version.ID, entryDigest(string(body)), previous)
	entryExec(t, `INSERT INTO pangu.catalog_version_bodies(version_id,spec) VALUES($1,$2)`, id, body)
	entryExec(t, `INSERT INTO pangu.catalog_version_state(version_id,state) VALUES($1,$2)`, id, state)
	return id
}

func (f entryFixture) adopt(t *testing.T, version string) {
	t.Helper()
	entryExec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$2,revision=revision+1 WHERE namespace=$1`, f.actor.Namespace, version)
}

func TestEntryUsesCurrentObservingButNeverExplicitHistory(t *testing.T) {
	f := newEntryFixture(t)
	old := f.automatic(t, f.project, "stable")
	current := f.automatic(t, f.project, "observing")
	f.adopt(t, current)
	got, err := f.submit("task", "adopted")
	if err != nil || got.Snapshot.Workflow.VersionID != current || got.Snapshot.EntryRevision != 2 {
		t.Fatal("current observing not selected", got, err)
	}
	if _, err = catalog.Compile(f.ctx, f.actor.Namespace, current, f.project, "task"); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatal("observing became explicit version", err)
	}
	explicit, err := catalog.Compile(f.ctx, f.actor.Namespace, old, f.project, "task")
	if err != nil {
		t.Fatal(err)
	}
	record, err := submission.Prepare(f.ctx, f.actor.Namespace, "submit", "adopted", submission.Snapshot{Workflow: *explicit})
	if err != nil || record.Snapshot.EntryKey != "" || record.Snapshot.EntryRevision != 0 || record.ID == got.ID {
		t.Fatal("explicit version marked as entry", record, err)
	}
	if _, err = submitEntry(f.ctx, f.actor.Namespace, f.project, old, "task", "history", entryHooks{entryReady, entryPending}); err == nil {
		t.Fatal("historical version selected via entry ID")
	}
	f.adopt(t, old)
	stable, err := f.submit("task", "stable-fallback")
	if err != nil || stable.Snapshot.Workflow.VersionID != old {
		t.Fatal("stable binding fallback failed", stable, err)
	}
	f.adopt(t, f.version.ID)
	fallback, err := f.submit("task", "fallback")
	if err != nil || fallback.Snapshot.Workflow.VersionID != f.version.ID {
		t.Fatal("manual binding fallback failed", fallback, err)
	}
	retry, err := f.submit("task", "adopted")
	if err != nil || retry.ID != got.ID || retry.Snapshot.Workflow.VersionID != current {
		t.Fatal("retry changed frozen adopted version", retry, err)
	}
}

func TestEntryRejectsForeignRevokedAndTombstonedAutomatic(t *testing.T) {
	for _, state := range []string{"foreign", "revoked", "tombstoned"} {
		t.Run(state, func(t *testing.T) {
			f := newEntryFixture(t)
			project := f.project
			if state == "foreign" {
				project = uuid.NewString()
				entryExec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'other','unused','main','test','git')`, project, f.actor.Namespace)
			}
			status := "revoked"
			if state == "foreign" {
				status = "observing"
			}
			id := f.automatic(t, project, status)
			if state == "tombstoned" {
				entryExec(t, `UPDATE pangu.catalog_version_bodies SET spec=NULL,cleared_at=now() WHERE version_id=$1`, id)
			}
			f.adopt(t, id)
			if _, err := f.submit("task", "bad-version"); !errors.Is(err, catalog.ErrNotFound) {
				t.Fatal("invalid automatic accepted", err)
			}
			if n := entrySubmissionCount(t, f); n != 0 {
				t.Fatal("invalid version persisted", n)
			}
		})
	}
}

func TestEntryLifecycleRevocationDuringPreflightIsRejected(t *testing.T) {
	f := newEntryFixture(t)
	current := f.automatic(t, f.project, "observing")
	f.adopt(t, current)
	check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
		entryExec(t, `UPDATE pangu.catalog_version_state SET state='revoked',revision=revision+1 WHERE version_id=$1`, current)
		return &workflow.Preflight{Ready: true}, nil
	}
	_, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "revocation", entryHooks{check, entryPending})
	if !errors.Is(err, catalog.ErrNotFound) {
		t.Fatal("revoked version accepted after compile", err)
	}
	if n := entrySubmissionCount(t, f); n != 0 {
		t.Fatal("revoked version persisted", n)
	}
}
