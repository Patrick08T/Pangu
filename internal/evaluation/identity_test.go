package evaluation

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"strings"
	"testing"
	"time"
)

func TestFeedbackRechecksRevocationAfterWaitingForRetention(t *testing.T) {
	ns, project, run, user := evaluationFixture(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
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
		_, err := SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, FeedbackInput{Decision: "accepted", Reason: "人工验收", SubjectHash: strings.Repeat("a", 64)})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("未等待保留锁: %v", err)
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
			t.Fatalf("撤权后仍采纳 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("写入未结束")
	}
}
