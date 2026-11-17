package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func (f entryFixture) runner(t *testing.T) identity.Actor {
	t.Helper()
	actor := identity.Actor{ID: uuid.NewString(), Namespace: f.actor.Namespace}
	entryExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor.ID, actor.Namespace)
	entryExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'operator',true)`, actor.ID, f.project)
	return actor
}

func TestEntryCurrentOperatorCanRunButCannotReplayAnotherActor(t *testing.T) {
	f := newEntryFixture(t)
	first, err := f.submit("task", "owner")
	if err != nil {
		t.Fatal(err)
	}
	actor := f.runner(t)
	ctx := identity.WithActor(context.Background(), actor)
	got, err := submitEntry(ctx, actor.Namespace, f.project, "main", "task", "operator", entryHooks{entryReady, entryPending})
	if err != nil || got.CreatedBy != actor.ID {
		t.Fatal("authorized operator denied", got, err)
	}
	if _, err = submitEntry(ctx, actor.Namespace, f.project, "main", "task", "owner", entryHooks{entryReady, entryPending}); !errors.Is(err, submission.ErrConflict) {
		t.Fatal("another actor could replay acceptance", first.ID, err)
	}
	entryExec(t, `UPDATE pangu.memberships SET role='viewer' WHERE user_id=$1`, f.actor.ID)
	if _, err = submitEntry(ctx, actor.Namespace, f.project, "main", "task", "revoked-policy-owner", entryHooks{entryReady, entryPending}); !errors.Is(err, identity.ErrForbidden) {
		t.Fatal("revoked policy owner still grants authority", err)
	}
}

func TestEntryTokenRunScopeAndLiveRevocation(t *testing.T) {
	for _, kind := range []string{"valid", "scope", "project", "revoked", "expired", "preflight-revoked"} {
		t.Run(kind, func(t *testing.T) {
			f := newEntryFixture(t)
			actor := f.runner(t)
			actor.TokenID = uuid.NewString()
			actor.ProjectID = f.project
			actor.Scopes = []string{"run"}
			entryExec(t, `INSERT INTO pangu.api_tokens(id,token_hash,user_id,project_id,name,scopes,expires_at) VALUES($1,$2,$3,$4,'entry test',ARRAY['run'],now()+interval '1 hour')`, actor.TokenID, entryDigest(actor.TokenID), actor.ID, f.project)
			switch kind {
			case "scope":
				entryExec(t, `UPDATE pangu.api_tokens SET scopes=ARRAY['read'] WHERE id=$1`, actor.TokenID)
			case "project":
				actor.ProjectID = uuid.NewString()
			case "revoked":
				entryExec(t, `UPDATE pangu.api_tokens SET revoked_at=now() WHERE id=$1`, actor.TokenID)
			case "expired":
				entryExec(t, `UPDATE pangu.api_tokens SET expires_at=now()-interval '1 second' WHERE id=$1`, actor.TokenID)
			}
			check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
				if kind == "preflight-revoked" {
					entryExec(t, `UPDATE pangu.api_tokens SET revoked_at=now() WHERE id=$1`, actor.TokenID)
				}
				return &workflow.Preflight{Ready: true}, nil
			}
			_, err := submitEntry(identity.WithActor(f.ctx, actor), actor.Namespace, f.project, "main", "task", "token", entryHooks{check, entryPending})
			if kind == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, identity.ErrForbidden) {
				t.Fatal("invalid token accepted", err)
			}
			if n := entrySubmissionCount(t, f); n != 0 {
				t.Fatal("unauthorized token persisted", n)
			}
		})
	}
}

func TestEntryMissingPolicyDoesNotEnableOrCreateBinding(t *testing.T) {
	f := newEntryFixture(t)
	project := uuid.NewString()
	entryExec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'unconfigured','unused','main','test','git')`, project, f.actor.Namespace)
	entryExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'manager',true)`, f.actor.ID, project)
	_, err := submitEntry(f.ctx, f.actor.Namespace, project, "main", "task", "off", entryHooks{entryReady, entryPending})
	if !errors.Is(err, identity.ErrForbidden) {
		t.Fatal("missing policy enabled entrance", err)
	}
	var policies, bindings int
	err = entryQuery(t, `SELECT (SELECT count(*) FROM pangu.experience_policies WHERE project_id=$1),(SELECT count(*) FROM pangu.evolution_bindings WHERE project_id=$1)`, project).Scan(&policies, &bindings)
	if err != nil || policies != 0 || bindings != 0 {
		t.Fatal("entry created authority", policies, bindings, err)
	}
}

func TestEntryRejectsNonBusinessReplay(t *testing.T) {
	f := newEntryFixture(t)
	d, err := catalog.Compile(f.ctx, f.actor.Namespace, f.version.ID, f.project, "task")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := submission.Snapshot{Purpose: "evaluation", Workflow: *d, EntryKey: "workbench:main", EntryRevision: 1}
	if _, err = submission.Prepare(f.ctx, f.actor.Namespace, "entry", "evaluation", snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = f.submit("task", "evaluation"); !errors.Is(err, submission.ErrConflict) {
		t.Fatal("evaluation was accepted as business entrance", err)
	}
}
