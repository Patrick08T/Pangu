package api

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/research"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func assertClaimReviewRejectsToken(t *testing.T, h http.Handler, project, path, body string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var user, ns string
	if err = conn.QueryRow(ctx, "SELECT m.user_id::text,p.namespace FROM pangu.memberships m JOIN pangu.projects p ON p.id=m.project_id WHERE p.id=$1", project).Scan(&user, &ns); err != nil {
		t.Fatal(err)
	}
	_, token, err := identity.IssueToken(ctx, identity.Actor{ID: user, Namespace: ns}, project, "自动核验", []string{"read", "manage_run"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("PUT", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("Token冒充人工核验 %d %s", w.Code, w.Body.String())
	}
}

func assertClaimRevocationAfterWait(t *testing.T, project, run, report, hash string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var user, ns string
	if err = conn.QueryRow(ctx, "SELECT m.user_id::text,p.namespace FROM pangu.memberships m JOIN pangu.projects p ON p.id=m.project_id WHERE p.id=$1", project).Scan(&user, &ns); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "retention:"+run); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := research.Review(ctx, ns, run, report, "c1", identity.Actor{ID: user, Namespace: ns}, research.ClaimReviewInput{Revision: 1, SubjectHash: hash, Verdict: "supported", Reason: "应被撤权阻止"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("未等待保留锁 %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.users SET disabled=true WHERE id=$1", user); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, identity.ErrForbidden) {
			t.Fatalf("撤权后仍核验 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("核验未结束")
	}
}
