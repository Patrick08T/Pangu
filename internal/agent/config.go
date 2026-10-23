package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

func checkCodexHome(workspace string) error {
	home := os.Getenv("PANGU_CODEX_HOME")
	if !filepath.IsAbs(home) {
		return errors.New("必须配置独立的 PANGU_CODEX_HOME 绝对目录，并由管理员登记登录、技能与 MCP")
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		return errors.New("Codex 专用配置目录不存在或不可访问")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return errors.New("Codex 专用配置必须是目录")
	}
	personal, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for _, forbidden := range []string{personal, filepath.Join(personal, ".codex"), os.Getenv("CODEX_HOME")} {
		if actual, err := filepath.EvalSymlinks(forbidden); err == nil && strings.EqualFold(resolved, actual) {
			return errors.New("Codex 专用配置不能复用宿主的个人配置目录")
		}
	}
	if actual, err := filepath.EvalSymlinks(workspace); err == nil {
		rel, err := filepath.Rel(actual, resolved)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("Codex 专用配置不能放在 Agent 可写的工作区内")
		}
	}
	return nil
}

func checkCodexPolicy(ctx context.Context, binary string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "exec")
	cmd.Dir = os.Getenv("PANGU_CODEX_HOME")
	output := diagnostic.Buffer{Limit: 1024}
	cmd.Stdin, cmd.Stdout, cmd.Env = strings.NewReader(""), &output, append(codexEnvironment(), "PANGU_CODEX_POLICY_PROBE=1")
	cmd.WaitDelay = time.Second
	if err := runCodexProcess(ctx, cmd, ""); err != nil || output.Truncated || strings.TrimSpace(output.Data.String()) != "pangu-codex-policy/1" {
		return errors.New("Codex 必须使用 Pangu 受控构建，普通 CLI 不具备工具次数与单次时限硬限制")
	}
	return nil
}

func codexEnvironment() []string {
	var env []string
	for _, name := range []string{"PATH", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	home := os.Getenv("PANGU_CODEX_HOME")
	return append(env, "CODEX_HOME="+home, "HOME="+home, "USERPROFILE="+home)
}

func codexIsolationArgs(workspace string) []string {
	args := []string{"--strict-config", "--ignore-rules", "-c", "projects." + strconv.Quote(workspace) + `.trust_level="untrusted"`,
		"-c", `web_search="disabled"`, "-c", "allow_login_shell=false", "-c", "agents.enabled=false", "-c", "notify=[]", "-c", "features.skip_host_skill_discovery=true"}
	for _, feature := range []string{"apps", "plugins", "remote_plugin", "multi_agent", "multi_agent_v2", "skill_search", "skill_mcp_dependency_install", "image_generation", "hooks"} {
		args = append(args, "--disable", feature)
	}
	if runtime.GOOS == "windows" {
		args = append(args, "-c", `windows.sandbox="unelevated"`)
	}
	return args
}
