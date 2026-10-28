package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func executeCodex(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	parents, err := research.ParentContext(ctx, input)
	if err != nil {
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "parameter", Cause: err}
	}
	input.Parents = parents
	return runCodex(ctx, input, "", nil)
}

func runCodex(ctx context.Context, input workflow.NodeInput, outputSchema string, search *codexSearch) (workflow.NodeOutput, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	binary, workspace, err := codexPaths(input.Workspace)
	if err == nil {
		err = checkCodexPolicy(ctx, binary)
	}
	if err != nil {
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "parameter", Cause: err}
	}
	output, err := os.CreateTemp("", "pangu-codex-*.txt")
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer os.Remove(output.Name())
	if err := output.Close(); err != nil {
		return workflow.NodeOutput{}, err
	}
	prompt, err := json.Marshal(input)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	sandbox := "workspace-write"
	if input.Node.ReadOnly {
		sandbox = "read-only"
	}
	args := []string{"exec", "--sandbox", sandbox, "-c", `approval_policy="never"`,
		"--cd", workspace, "--ephemeral", "--json", "--color", "never", "--output-last-message", output.Name()}
	args = append(args, codexIsolationArgs(workspace)...)
	if outputSchema != "" {
		args = append(args, "--output-schema", outputSchema, "--skip-git-repo-check")
	}
	if input.Node.Model != "" {
		args = append(args, "--model="+input.Node.Model)
	}
	cmd := exec.CommandContext(ctx, binary, append(args, "-")...)
	cmd.Dir, cmd.Stdin = workspace, bytes.NewReader(prompt)
	log := &diagnostic.Buffer{Limit: 1024 * 1024}
	events := &codexEvents{Search: search, Context: ctx, Cancel: cancel, Prefix: uuid.NewString() + "/"}
	cmd.Stdout = events
	cmd.Env, cmd.Stderr = append(codexEnvironment(), "PANGU_CODEX_TRACE=1"), log
	if outputSchema != "" && search == nil {
		cmd.Env = append(cmd.Env, "PANGU_CODEX_MODEL_ONLY=1")
	}
	if search != nil {
		search.configure(cmd)
	}
	cmd.WaitDelay = 3 * time.Second
	runErr := runCodexUsage(ctx, cmd, input.ProcessScope, input.Node.Model, events)
	result := workflow.NodeOutput{Evidence: events.Commands, EvidenceTruncated: events.Truncated}
	if err := codexRunError(ctx, runErr, events, log); err != nil {
		return result, err
	}
	limit := 16384
	if outputSchema != "" {
		limit = 1024 * 1024
	}
	result.Text, err = codexOutput(output.Name(), limit)
	return result, err
}

func codexRunError(ctx context.Context, runErr error, events *codexEvents, log *diagnostic.Buffer) error {
	if events.TraceErr != nil && !errors.Is(runErr, workflow.ErrStopUnconfirmed) {
		return &workflow.Failure{Kind: "trace", Cause: events.TraceErr}
	}
	if runErr == nil {
		return nil
	}
	message, truncated := diagnostic.Text(log.Data.String(), 64*1024)
	cause := fmt.Errorf("Codex 执行失败: %w；事件=%s；诊断=%s；截断=%t", errors.Join(runErr, events.TraceErr), events.ErrorMessage, message, truncated || log.Truncated)
	if ctx.Err() != nil {
		return errors.Join(cause, ctx.Err())
	}
	return codexFailure(cause, events.ErrorStatus)
}

func codexOutput(path string, limit int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if len(data) > limit {
		return "", &workflow.Failure{Kind: "output", Cause: fmt.Errorf("Codex 回复超过 %d 字节", limit)}
	}
	return string(data), err
}

func codexPaths(workspace string) (string, string, error) {
	if err := checkCodexHome(workspace); err != nil {
		return "", "", err
	}
	binary := os.Getenv("PANGU_CODEX_BINARY")
	if !filepath.IsAbs(binary) || !filepath.IsAbs(workspace) {
		return "", "", fmt.Errorf("Codex 程序和服务端分配的工作目录必须为绝对路径")
	}
	info, err := os.Stat(binary)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("Codex 程序路径必须为文件")
	}
	info, err = os.Stat(workspace)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("Codex 工作目录不存在")
	}
	return binary, workspace, nil
}
