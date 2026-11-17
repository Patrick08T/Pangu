package scheduler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func loopRepairDefinition(project string) workflow.Definition {
	ids := []string{"discover", "reproduce", "fix", "selftest", "review", "qa"}
	d := workflow.Definition{ProjectID: project, Loops: []workflow.Loop{{ID: "bugs", Nodes: ids, Gates: ids[3:]}}}
	for i, id := range ids {
		n := workflow.Node{ID: id, Executor: "codex", Instruction: id, Handoff: testHandoff(), Policy: &workflow.Policy{Retries: 0}}
		if i > 0 {
			n.DependsOn = []string{ids[i-1]}
		}
		if i >= 3 {
			n.ReadOnly, n.OutputSchema = true, workflow.LoopGateSchema()
		}
		d.Nodes = append(d.Nodes, n)
	}
	return d
}

func TestHatchetLoopRestartRetainsPatchRoundsAndModelRetry(t *testing.T) {
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	p := registerTestProject(t, b)
	var mu sync.Mutex
	counts := map[string]int{}
	execute := func(_ context.Context, in workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		key := fmt.Sprintf("%d/%s", in.Iteration.Round, in.Node.ID)
		counts[key]++
		if key == "2/review" && counts[key] == 1 {
			return workflow.NodeOutput{}, &workflow.Failure{Kind: "model_unavailable", Retry: true, Cause: errors.New("测试模型临时容量不足")}
		}
		return repairTestAgent(in)
	}
	stop := startReviewWorker(t, client, execute)
	t.Cleanup(func() {
		if stop != nil {
			_ = stop()
		}
	})
	r, err := b.Submit(context.Background(), loopRepairDefinition(p.ID), "loop-restart")
	if err != nil {
		t.Fatal(err)
	}
	awaitModelFailure(t, b, r.RunID)
	if err = stop(); err != nil {
		t.Fatal(err)
	}
	stop = nil
	stop = startReviewWorker(t, client, execute)
	out := waitRun(t, api.New(b, integrationAccess{}), r.RunID)
	mu.Lock()
	defer mu.Unlock()
	for key, count := range counts {
		if count != 1 && !(key == "2/review" && count == 2) {
			t.Fatalf("已完成任务重复执行 %s %d", key, count)
		}
	}
	if counts["1/review"] != 0 || counts["1/qa"] != 0 || counts["2/qa"] != 1 {
		t.Fatalf("错误的返修范围 %v", counts)
	}
	if len(out.Nodes) != 11 {
		t.Fatalf("轮次历史丢失: %d", len(out.Nodes))
	}
	for _, n := range out.Nodes {
		if n.LoopID != "bugs" || n.LoopRound < 1 {
			t.Fatalf("缺少轮次: %+v", n)
		}
	}
	assertLoopHistory(t, b, &out)
}

func assertLoopHistory(t *testing.T, b scheduler.Backend, run *scheduler.RunDetails) {
	t.Helper()
	body, err := submission.History(context.Background(), b.Namespace(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var archived scheduler.RunDetails
	if err = json.Unmarshal(body, &archived); err != nil {
		t.Fatal(err)
	}
	if len(archived.Nodes) != len(run.Nodes) {
		t.Fatal("终态摘要丢失节点历史")
	}
	for _, node := range archived.Nodes {
		if node.LoopID != "bugs" || node.LoopRound < 1 {
			t.Fatal("日志到期后的元数据丢失循环轮次")
		}
		if node.Output != "" || node.Result != nil {
			t.Fatal("元数据归档不应保留已过期的正文")
		}
	}
}

func repairTestAgent(in workflow.NodeInput) (workflow.NodeOutput, error) {
	if in.Iteration.Round == 2 && in.Node.ID == "discover" {
		data, err := os.ReadFile(filepath.Join(in.Workspace, "patch.txt"))
		if err != nil || string(data) != "1" || in.Iteration.Feedback == nil {
			return workflow.NodeOutput{}, errors.New("重试丢失当前补丁或反馈")
		}
	}
	if in.Node.ID == "fix" {
		if err := os.WriteFile(filepath.Join(in.Workspace, "patch.txt"), []byte(fmt.Sprint(in.Iteration.Round)), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
	}
	if in.Iteration.Gate {
		return workflow.NodeOutput{Text: fmt.Sprintf(`{"passed":%t,"summary":"独立检查结论"}`, in.Iteration.Round > 1)}, nil
	}
	return workflow.NodeOutput{Text: "阶段完成"}, nil
}
