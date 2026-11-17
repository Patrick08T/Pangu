//go:build integration

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
)

type experienceFixture struct {
	handler                                   http.Handler
	namespace, project, other, actor, session string
}

// This fixture never migrates: the coordinated integration database is already frozen.
func experienceAPI(t *testing.T) experienceFixture {
	t.Helper()
	value := os.Getenv("PANGU_TEST_DATABASE_URL")
	if _, err := parseExperienceTestDatabase(value); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_DATABASE_URL", value)
	if err := database.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"PANGU_FEISHU_APP_ID": "test", "PANGU_FEISHU_APP_SECRET": "test", "PANGU_FEISHU_TENANT_KEY": "tenant", "PANGU_PUBLIC_URL": "https://pangu.example"} {
		t.Setenv(k, v)
	}
	f := experienceFixture{namespace: uuid.NewString(), project: uuid.NewString(), other: uuid.NewString(), actor: uuid.NewString(), session: uuid.NewString() + "session"}
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, id := range []string{f.project, f.other} {
		if _, err = conn.Exec(ctx, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,$2,'test','test','main','worker')`, id, f.namespace); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1,$2,'test','tenant','ou_experience')`, f.actor, f.namespace); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.memberships(user_id,project_id,role) VALUES($1,$2,'manager')`, f.actor, f.project); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(f.session))
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '1 hour')`, hex.EncodeToString(hash[:]), f.actor); err != nil {
		t.Fatal(err)
	}
	f.handler = New(&authorizedBackend{namespace: f.namespace}, identity.Service{Namespace: f.namespace})
	return f
}

func (f experienceFixture) role(t *testing.T, role string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `UPDATE pangu.memberships SET role=$1 WHERE user_id=$2 AND project_id=$3`, role, f.actor, f.project); err != nil {
		t.Fatal(err)
	}
}
