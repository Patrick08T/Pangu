//go:build integration

package identity

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func authorizationErrorFixture(t *testing.T) (context.Context, *pgx.Conn, Actor, string) {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("authorization integration requires isolated PANGU_TEST_DATABASE_URL")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	actor := Actor{ID: uuid.NewString(), Namespace: uuid.NewString()}
	project := uuid.NewString()
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id)
	 VALUES($1,$2,'test','test',$3)`, actor.ID, actor.Namespace, actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs)
	 VALUES($1,$2,'authorization-error','unused','main','test','git')`, project, actor.Namespace); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.memberships(user_id,project_id,role)
	 VALUES($1,$2,'manager')`, actor.ID, project); err != nil {
		t.Fatal(err)
	}
	return ctx, conn, actor, project
}

func TestAuthorizePreservesDatabaseQueryErrors(t *testing.T) {
	ctx, _, actor, project := authorizationErrorFixture(t)
	if err := Authorize(ctx, actor, project, "read", ""); err != nil {
		t.Fatal("valid actor denied", err)
	}
	err := Authorize(ctx, actor, "not-a-project-uuid", "read", "")
	var sqlErr *pgconn.PgError
	if errors.Is(err, ErrForbidden) || !errors.As(err, &sqlErr) || sqlErr.Code != "22P02" {
		t.Fatalf("query failure lost its database cause: %v", err)
	}
}

func TestAuthorizeErrorHandlingPreservesDenials(t *testing.T) {
	ctx, conn, actor, project := authorizationErrorFixture(t)
	missing := actor
	missing.ID = uuid.NewString()
	wrongNamespace := actor
	wrongNamespace.Namespace = uuid.NewString()
	token := actor
	token.TokenID, token.ProjectID, token.Scopes = uuid.NewString(), project, []string{"read"}
	for _, tc := range []struct {
		name, project, action string
		actor                 Actor
	}{
		{"missing actor", project, "read", missing},
		{"namespace mismatch", project, "read", wrongNamespace},
		{"missing project", uuid.NewString(), "read", actor},
		{"token scope", project, "configure", token},
		{"token project", uuid.NewString(), "read", token},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Authorize(ctx, tc.actor, tc.project, tc.action, ""); !errors.Is(err, ErrForbidden) {
				t.Fatal("denial no longer forbidden", err)
			}
		})
	}
	if _, err := conn.Exec(ctx, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err := Authorize(ctx, actor, project, "read", ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("removed membership allowed", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE pangu.users SET disabled=true WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err := Authorize(ctx, actor, "", "read_catalog", ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("disabled actor allowed", err)
	}
}
