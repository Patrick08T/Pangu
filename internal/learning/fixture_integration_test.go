//go:build integration

package learning

import (
	"context"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type learningFixture struct {
	ctx   context.Context
	scope experience.Scope
	actor identity.Actor
	store experience.Store
}

func newLearningFixture(t *testing.T) learningFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("learning integration requires isolated PANGU_TEST_DATABASE_URL")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	actor := identity.Actor{ID: uuid.NewString(), Namespace: uuid.NewString()}
	f := learningFixture{ctx: identity.WithActor(context.Background(), actor), actor: actor,
		scope: experience.Scope{Namespace: actor.Namespace, ActorID: actor.ID, ProjectID: uuid.NewString()}, store: NewStore()}
	f.exec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1,$2,'test','test',$3)`, actor.ID, actor.Namespace, actor.ID)
	f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs)
	 VALUES($1,$2,'learning','unused','main','test','git')`, f.scope.ProjectID, actor.Namespace)
	f.exec(t, `INSERT INTO pangu.memberships(user_id,project_id,role) VALUES($1,$2,'manager')`, actor.ID, f.scope.ProjectID)
	return f
}

func (f learningFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(f.ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (f learningFixture) transaction(t *testing.T) pgx.Tx {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	return tx
}

func (f learningFixture) token(t *testing.T, scopes []string) context.Context {
	t.Helper()
	actor := f.actor
	actor.TokenID, actor.ProjectID, actor.Scopes = uuid.NewString(), f.scope.ProjectID, scopes
	f.exec(t, `INSERT INTO pangu.api_tokens(id,token_hash,user_id,project_id,name,scopes,expires_at)
	 VALUES($1,$5,$2,$3,'learning-test',$4,now()+interval '1 hour')`, actor.TokenID, actor.ID, actor.ProjectID, scopes, actor.TokenID)
	return identity.WithActor(f.ctx, actor)
}
