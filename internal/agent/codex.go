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
	"strconv"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func executeCodex(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Minute)
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
	if input.Node.Model != "" {
		args = append(args, "--model="+input.Node.Model)
	}
	cmd := exec.CommandContext(ctx, binary, append(args, "-")...)
	cmd.Dir, cmd.Stdin = workspace, bytes.NewReader(prompt)
	log := &diagnostic.Buffer{Limit: 1024 * 1024}
	events := &codexEvents{}
	cmd.Stdout = events
	cmd.Env, cmd.Stderr = append(codexEnvironment(), "PANGU_CODEX_TOOL_CALLS="+strconv.Itoa(input.Node.EffectivePolicy().ToolCalls)), log
	cmd.WaitDelay = 3 * time.Second
	runErr := runCodexProcess(ctx, cmd, input.ProcessScope)
	result := workflow.NodeOutput{Evidence: events.Commands, EvidenceTruncated: events.Truncated}
	if runErr != nil {
		message, truncated := diagnostic.Text(log.Data.String(), 64*1024)
		cause := fmt.Errorf("Codex 执行失败: %w；事件=%s；诊断=%s；截断=%t", runErr, events.ErrorMessage, message, truncated || log.Truncated)
		if ctx.Err() != nil {
			return result, errors.Join(cause, ctx.Err())
		}
		return result, codexFailure(cause, events.ErrorMessage)
	}
	file, err := os.Open(output.Name())
	if err != nil {
		return result, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil {
		return result, err
	}
	result.Text = string(data)
	return result, nil
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
