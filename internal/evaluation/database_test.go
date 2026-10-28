package evaluation

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func evaluationFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	ns, project, user := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,$2,'评测试点','unused','main','test')`, project, ns); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,'test','test',$3,true)`, user, ns, user); err != nil {
		t.Fatal(err)
	}
	record, err := submission.Prepare(ctx, ns, "submit", "one", submission.Snapshot{Workflow: workflow.Definition{ProjectID: project}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, ns, record.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	return ns, project, run, user
}

func TestFeedbackRevisionPreservesFirstVerdictAndRejectsStaleArtifact(t *testing.T) {
	ns, project, run, user := evaluationFixture(t)
	ctx := context.Background()
	input := FeedbackInput{Decision: "needs_changes", Reason: "缺少测试证据", SubjectHash: strings.Repeat("a", 64)}
	first, err := SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, input)
	if err != nil || again.Revision != 1 || again.ID != first.ID {
		t.Fatalf("重试重复计数: %+v %v", again, err)
	}
	input.Decision, input.Revision = "accepted", 1
	changed, err := SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, input)
	if err != nil || changed.FirstDecision != "needs_changes" || changed.Decision != "accepted" {
		t.Fatalf("初次评价被改写: %+v %v", changed, err)
	}
	input.Revision, input.SubjectHash = 2, strings.Repeat("b", 64)
	if _, err = SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, input); !errors.Is(err, ErrConflict) {
		t.Fatalf("产物变化被忽略: %v", err)
	}
	if _, err = FeedbackForRun(ctx, "foreign", run); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨空间泄漏: %v", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "SELECT pangu.purge_run_payloads($1)", run); err != nil {
		t.Fatal(err)
	}
	saved, err := FeedbackForRun(ctx, ns, run)
	if err != nil || saved.Reason != "" || !saved.PayloadExpired {
		t.Fatalf("评价正文未清理: %+v %v", saved, err)
	}
	if _, err = SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, input); !errors.Is(err, ErrExpired) {
		t.Fatalf("迟到写入恢复过期理由: %v", err)
	}
}

func TestConcurrentFeedbackOnlyAcceptsOneRevision(t *testing.T) {
	ns, project, run, user := evaluationFixture(t)
	ctx := context.Background()
	var group sync.WaitGroup
	results := make(chan error, 2)
	for _, decision := range []string{"accepted", "rejected"} {
		group.Go(func() {
			_, err := SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, FeedbackInput{Decision: decision, Reason: decision, SubjectHash: strings.Repeat("a", 64)})
			results <- err
		})
	}
	group.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("并发覆盖: success=%d conflict=%d", success, conflict)
	}
}

func TestQualityCountsRerunsOnceAndKeepsInitialVerdict(t *testing.T) {
	ns, project, run, user := evaluationFixture(t)
	ctx := context.Background()
	_, err := SaveFeedback(ctx, ns, project, run, identity.Actor{ID: user, Namespace: ns}, FeedbackInput{Decision: "needs_changes", Reason: "需要返工", SubjectHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	next, err := submission.Prepare(ctx, ns, "rerun", "second", submission.Snapshot{Workflow: workflow.Definition{ProjectID: project}, SourceRunID: run})
	if err != nil {
		t.Fatal(err)
	}
	rerun := uuid.NewString()
	if _, err = submission.Dispatch(ctx, ns, next.ID, func(context.Context, *submission.Record) (string, error) { return rerun, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = SaveFeedback(ctx, ns, project, rerun, identity.Actor{ID: user, Namespace: ns}, FeedbackInput{Decision: "accepted", Reason: "复核通过", SubjectHash: strings.Repeat("b", 64)}); err != nil {
		t.Fatal(err)
	}
	result, err := ProjectQuality(ctx, ns, project)
	if err != nil || result.Tasks != 1 || result.Runs != 2 || result.FirstAccepted != 0 || result.FirstReviewed != 1 || result.AcceptedTasks != 1 {
		t.Fatalf("质量分母错误: %+v %v", result, err)
	}
}
