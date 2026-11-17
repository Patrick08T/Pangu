package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func TestHatchetModelRetryPreservesHumanCorrectionAndRevisionReports(t *testing.T) {
	testModelRetryReview(t, false)
}

func TestHatchetModelRetryRecoversFailedRevisionWithoutRepeatingPlans(t *testing.T) {
	testModelRetryReview(t, true)
}

func testModelRetryReview(t *testing.T, legacy bool) {
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	p := registerTestProject(t, b)
	actor := approvalActor(t, b.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	var mu sync.Mutex
	calls := map[string]int{}
	stop := startReviewWorker(t, client, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[input.Node.ID]++
		if f := input.ReviewFeedback; f != nil {
			if f.Round != 2 || len(f.Items) != 1 || f.Items[0].Correction != "仅交付文档" {
				return workflow.NodeOutput{}, errors.New("人工纠偏丢失")
			}
			if input.Node.ID == "plan" && calls["plan"] == 2 {
				if legacy {
					return workflow.NodeOutput{}, errors.New("旧版没有识别模型容量不足")
				}
				return workflow.NodeOutput{}, &workflow.Failure{Kind: "model_unavailable", Retry: true, Cause: errors.New("容量不足")}
			}
			if input.Node.ID == "plan" && input.Parents["source"].Text != revisionReport(true) {
				return workflow.NodeOutput{}, errors.New("修订上游报告丢失")
			}
		}
		return workflow.NodeOutput{Text: revisionReport(input.ReviewFeedback != nil)}, nil
	})
	defer stop()
	d := retryReviewDefinition(p.ID)
	r, err := b.Submit(ctx, d, "revision-model-retry")
	if err != nil {
		t.Fatal(err)
	}
	first := awaitReviewRound(t, b, r.RunID, 1)
	_, err = approval.Decide(ctx, actor, first.ID, approval.Decision{SubjectHash: first.SubjectHash, Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise", Correction: "仅交付文档"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = b.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	if legacy {
		recoverFailedRevision(t, b, client, r.RunID)
	}
	second := awaitReviewRound(t, b, r.RunID, 2)
	mu.Lock()
	correct := calls["source"] == 2 && calls["plan"] == 3 && calls["build"] == 0
	mu.Unlock()
	if !correct || second.Status != "waiting" || second.PreviousID != first.ID {
		t.Fatalf("纠偏重试未保留状态: calls=%v review=%+v", calls, second)
	}
	if _, err = b.CancelRun(ctx, r.RunID); err != nil {
		t.Fatal(err)
	}
}

func retryReviewDefinition(project string) workflow.Definition {
	d := approvalWorkflow(project, "test")
	source := d.Nodes[0]
	source.ID = "source"
	d.Nodes[0].DependsOn = []string{"source"}
	d.Nodes[1].Approval.RevisionNodes = []string{"source", "plan"}
	d.Nodes = append([]workflow.Node{source}, d.Nodes...)
	for i := range d.Nodes {
		d.Nodes[i].Policy = &workflow.Policy{Retries: 0}
	}
	return d
}

// 模拟已核验原始提供方事件的一次性运维修复，不在产品中兼容旧错误正文。
func recoverFailedRevision(t *testing.T, b scheduler.Backend, client *hatchet.Client, run string) {
	failed := waitFailedRun(t, b, run)
	var node string
	for _, n := range failed.Nodes {
		if n.NodeID == "plan" && n.ReviewRound == 2 {
			node = n.ID
		}
	}
	if node == "" {
		t.Fatal("没有定位到失败修订节点")
	}
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "UPDATE pangu.node_attempts SET category='model_unavailable' WHERE run_id=$1 AND node_run_id=$2 AND category='execution' AND status='failed'", run, node)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{uuid.MustParse(run)}
	if _, err = client.Runs().Replay(ctx, rest.V1ReplayTaskRequest{ExternalIds: &ids}); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, "DELETE FROM pangu.run_history WHERE run_id=$1", run)
	if err != nil {
		t.Fatal(err)
	}
}
