package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func executeCodex(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Minute)
	defer cancel()
	binary, workspace, err := codexPaths()
	if err != nil {
		return workflow.NodeOutput{}, err
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
	args := []string{"exec", "--sandbox", "workspace-write", "-c", `approval_policy="never"`,
		"--cd", workspace, "--ephemeral", "--color", "never", "--output-last-message", output.Name()}
	if input.Node.Model != "" {
		args = append(args, "--model="+input.Node.Model)
	}
	cmd := exec.CommandContext(ctx, binary, append(args, "-")...)
	cmd.Dir, cmd.Stdin = workspace, bytes.NewReader(prompt)
	cmd.Env, cmd.Stderr = codexEnvironment(), os.Stderr
	cmd.WaitDelay = 3 * time.Second
	if err := runCodexProcess(ctx, cmd); err != nil {
		if ctx.Err() != nil {
			return workflow.NodeOutput{}, ctx.Err()
		}
		return workflow.NodeOutput{}, fmt.Errorf("Codex 执行失败（详细信息见 Worker 日志）: %w", err)
	}
	file, err := os.Open(output.Name())
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	return workflow.NodeOutput{Text: string(data)}, nil
}

func codexPaths() (string, string, error) {
	binary, workspace := os.Getenv("PANGU_CODEX_BINARY"), os.Getenv("PANGU_CODEX_WORKSPACE")
	if !filepath.IsAbs(binary) || !filepath.IsAbs(workspace) {
		return "", "", fmt.Errorf("PANGU_CODEX_BINARY 和 PANGU_CODEX_WORKSPACE 必须为绝对路径")
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

func codexEnvironment() []string {
	var env []string
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		key = strings.ToUpper(key)
		// 不传入编排服务令牌；Eino 的模型地址和凭据不能覆盖 Codex 登录配置。
		if !strings.HasPrefix(key, "HATCHET_") && !strings.HasPrefix(key, "PANGU_") && key != "OPENAI_API_KEY" && key != "OPENAI_BASE_URL" {
			env = append(env, value)
		}
	}
	return env
}
