package identity

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestTokenIssueWaitsForRevocationBoundaryAndRechecksPermission(t *testing.T) {
	ctx, ns := identityDB(t)
	actor, _, err := login(ctx, ns, Config{AppID: "test", Tenant: "tenant", AdminOpenID: "admin"}, FeishuUser{OpenID: "admin", Tenant: "tenant"})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	project := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,$2,'test','test','main','test')`, project, ns)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = Lock(ctx, tx, ns); err != nil {
		t.Fatal(err)
	}
	issueCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := IssueToken(issueCtx, actor, project, "blocked", []string{"read"}, time.Now().Add(time.Hour))
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("发放越过撤权事务: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err = tx.Exec(ctx, `UPDATE pangu.users SET disabled=true WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err == nil {
		t.Fatal("撤权后发放了有效 Token")
	}
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.api_tokens WHERE user_id=$1`, actor.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("延后插入了 Token: %d %v", count, err)
	}
}
