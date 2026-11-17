package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestLoopIdentitySeparatesBusinessRounds(t *testing.T) {
	in := workflow.NodeInput{Node: workflow.Node{ID: "fix", Executor: "codex"}, Iteration: &workflow.Iteration{ID: "bugs", Round: 1}}
	a, meta := nodeRunIdentity(in)
	in.Iteration.Round = 2
	b, _ := nodeRunIdentity(in)
	if a == b || meta["pangu_loop_id"] != "bugs" || meta["pangu_loop_round"] != "1" {
		t.Fatalf("轮次身份丢失 %s %s %v", a, b, meta)
	}
}

func TestLoopVerificationHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] != "pangu-loop-verify" {
		return
	}
	value, err := os.ReadFile("patch.txt")
	if err != nil || string(value) != "fixed-2" {
		os.Exit(7)
	}
	os.Exit(0)
}

func configureLoopVerification(t *testing.T, project string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	command, _ := json.Marshal([]string{program, "-test.run=^TestLoopVerificationHelper$", "--", "pangu-loop-verify"})
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.projects SET verification=$2 WHERE id=$1", project, command); err != nil {
		t.Fatal(err)
	}
}

func TestLoopKeepsPatchAndFailedVerificationForNextRound(t *testing.T) {
	run, project := revisionRun(t)
	configureLoopVerification(t, project)
	ids := []string{"discover", "reproduce", "fix", "selftest", "review", "qa"}
	d := workflow.Definition{ProjectID: project, Loops: []workflow.Loop{{ID: "bugs", Nodes: ids, Gates: ids[3:]}}}
	for i, id := range ids {
		n := workflow.Node{ID: id, Executor: "codex", Instruction: id}
		if i > 0 {
			n.DependsOn = []string{ids[i-1]}
		}
		if i >= 3 {
			n.ReadOnly, n.OutputSchema = true, workflow.LoopGateSchema()
		}
		d.Nodes = append(d.Nodes, n)
	}
	outputs, err := workflow.Run(d, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		in := inputs[0]
		if in.Iteration.Round > 2 {
			t.Fatal("二轮通过后不应继续")
		}
		item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), Attempt: 1, ProcessScope: uuid.NewString()}
		out, err := executeNode(context.Background(), item, in, func(_ context.Context, actual workflow.NodeInput) (workflow.NodeOutput, error) {
			return loopPatchAgent(t, actual), nil
		})
		if err == nil {
			_, saved, e := workspace.Checkpoint(context.Background(), run, item.NodeRunID)
			if e != nil || saved.Iteration == nil || saved.Iteration.Round != in.Iteration.Round {
				t.Fatalf("检查点未保留轮次：%+v %v", saved, e)
			}
		}
		return []workflow.NodeOutput{out}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if outputs["qa"].Iteration.Round != 2 || outputs["qa"].ContentRevision != outputs["selftest"].ContentRevision {
		t.Fatal("质量门未验证同一代码版本")
	}
}

func loopPatchAgent(t *testing.T, in workflow.NodeInput) workflow.NodeOutput {
	t.Helper()
	if in.Iteration.Round == 2 && in.Node.ID == "discover" {
		patch, err := os.ReadFile(filepath.Join(in.Workspace, "patch.txt"))
		if err != nil || string(patch) != "fixed-1" || in.Iteration.Feedback == nil || !strings.Contains(in.Iteration.Feedback.Summary, "7") {
			t.Fatal("返修丢失补丁或失败证据")
		}
	}
	if in.Node.ID == "fix" {
		if err := os.WriteFile(filepath.Join(in.Workspace, "patch.txt"), []byte(fmt.Sprintf("fixed-%d", in.Iteration.Round)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if in.Iteration.Gate {
		return workflow.NodeOutput{Text: `{"passed":true,"summary":"Agent检查完成"}`}
	}
	return workflow.NodeOutput{Text: "完成"}
}

func TestLoopCurrentRoundNeverReusesEarlierGate(t *testing.T) {
	now := time.Now()
	run := &RunDetails{Definition: &workflow.Definition{Nodes: []workflow.Node{{ID: "fix"}, {ID: "qa"}}, Loops: []workflow.Loop{{ID: "bugs", Nodes: []string{"fix", "qa"}, Gates: []string{"qa"}}}}, Nodes: []NodeRun{
		{NodeID: "fix", LoopID: "bugs", LoopRound: 2, RunSummary: RunSummary{Status: "running", CreatedAt: now}},
		{NodeID: "qa", LoopID: "bugs", LoopRound: 1, RunSummary: RunSummary{Status: "succeeded", CreatedAt: now.Add(-time.Hour)}, Output: `{"passed":true,"summary":"旧验收"}`},
	}}
	if _, ok := Outputs(run)["qa"]; ok {
		t.Fatal("前一轮验收仍显示为当前结果")
	}
	if _, ok := successfulNode(run, "qa"); ok {
		t.Fatal("前一轮验收仍允许检查点复用")
	}
}

func TestLoopProjectFailureUpdatesReportAndContentHash(t *testing.T) {
	body := `{"passed":true,"summary":"模型认为通过","details":"完整检查记录"}`
	out := workflow.NodeOutput{Text: `{"passed":true,"summary":"模型认为通过"}`, Reports: []workflow.Report{{Markdown: body}}, ReportSHA256: workflow.ReportHash(body)}
	if err := failedVerification(&out, 7); err != nil {
		t.Fatal(err)
	}
	passed, summary, err := workflow.GateVerdict(out)
	if err != nil || passed || !strings.Contains(summary, "7") {
		t.Fatalf("%t %s %v", passed, summary, err)
	}
	if out.ReportSHA256 != workflow.ReportHash(out.Reports[0].Markdown) || !strings.Contains(out.Reports[0].Markdown, `"passed":false`) || !strings.Contains(out.Reports[0].Markdown, "完整检查记录") {
		t.Fatal("完整报告与质量结论不一致")
	}
}
