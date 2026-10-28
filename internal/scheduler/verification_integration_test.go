package scheduler_test

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/artifact"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHatchetVerificationFailureCannotPublishOrRetry(t *testing.T) {
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	worker, err := scheduler.NewWorker(client, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		if input.Node.Instruction == "change" {
			if err := os.WriteFile(filepath.Join(input.Workspace, "README.md"), []byte("未通过验证的修改"), 0600); err != nil {
				return workflow.NodeOutput{}, err
			}
		}
		return workflow.NodeOutput{Text: "执行完成"}, nil
	}, func(context.Context, workflow.Definition) workflow.Preflight { return workflow.Preflight{Ready: true} })
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	awaitWorker(t, backend)
	original := registerTestProject(t, backend)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	p, err := backend.RegisterProject(context.Background(), workspace.Project{Name: "验证命令", Source: original.Source, Branch: "main", WorkerID: original.WorkerID, Verification: []string{git, "diff", "--exit-code", "--", "README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	h := api.New(backend, integrationAccess{})
	for _, instruction := range []string{"inspect", "change"} {
		var accepted struct {
			RunID string `json:"run_id"`
		}
		request(t, h, "POST", "/runs", workflow.Definition{ProjectID: p.ID, Nodes: []workflow.Node{{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: instruction}}}, 202, &accepted)
		var run scheduler.RunDetails
		if instruction == "change" {
			run = *waitFailedRun(t, backend, accepted.RunID)
		} else {
			run = waitRun(t, h, accepted.RunID)
		}
		assertVerificationResult(t, run, instruction)
	}
}

func assertVerificationResult(t *testing.T, run scheduler.RunDetails, instruction string) {
	t.Helper()
	attempt, err := execution.AttemptByID(context.Background(), run.ID, run.Nodes[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(attempt.Evidence) != 1 {
		t.Fatalf("缺少验证证据: %+v", attempt)
	}
	if instruction == "inspect" && (run.Status != "succeeded" || attempt.Evidence[0].ExitCode != 0) {
		t.Fatalf("成功验证失败: %+v", run)
	}
	if instruction == "change" && (run.Status != "failed" || attempt.Category != "verification" || attempt.Evidence[0].ExitCode != 1 || len(run.Attempts) != 1) {
		t.Fatalf("失败验证未阻断或发生重试: %+v %+v", run, attempt)
	}
	files, err := artifact.List(context.Background(), run.ID)
	if err != nil || len(files.Files) != 0 {
		t.Fatalf("发布了未通过验证的文件: %+v %v", files, err)
	}
}

func TestHatchetVerificationHasNoNodeDeadline(t *testing.T) {
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	worker, err := scheduler.NewWorker(client, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "执行完成"}, nil
	}, func(context.Context, workflow.Definition) workflow.Preflight { return workflow.Preflight{Ready: true} })
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	awaitWorker(t, backend)
	original := registerTestProject(t, backend)
	program, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("需要 Windows PowerShell 验证子进程")
	}
	p, err := backend.RegisterProject(context.Background(), workspace.Project{Name: "无执行时限验证", Source: original.Source, Branch: "main", WorkerID: original.WorkerID, Verification: []string{program, "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 10"}})
	if err != nil {
		t.Fatal(err)
	}
	h := api.New(backend, integrationAccess{})
	var accepted struct {
		RunID string `json:"run_id"`
	}
	definition := workflow.Definition{ProjectID: p.ID, Nodes: []workflow.Node{{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: "inspect", Policy: &workflow.Policy{Retries: 2}}}}
	request(t, h, "POST", "/runs", definition, 202, &accepted)
	run := waitRun(t, h, accepted.RunID)
	if len(run.Attempts) != 1 || run.Attempts[0].Status != "succeeded" {
		t.Fatalf("验证仍受节点时限约束: %+v", run.Attempts)
	}
}
