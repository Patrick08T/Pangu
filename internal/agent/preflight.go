package agent

import (
	"context"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func Preflight(ctx context.Context, definition workflow.Definition) workflow.Preflight {
	out := workflow.Preflight{Ready: true, Checks: []workflow.Check{}}
	if err := definition.ValidateInput(); err != nil {
		return workflow.Preflight{Checks: []workflow.Check{{Component: "input", Status: "unavailable"}}}
	}
	for _, node := range definition.Nodes {
		if node.Role == "" {
			continue
		}
		brief, _ := workflow.ParseBrief(definition.Input, node.Mode)
		if len(brief.PublicQueries) > 0 {
			status := codexReady(ctx)
			out.Ready = out.Ready && status != "unavailable"
			out.Checks = append(out.Checks, workflow.Check{Component: "search", Status: status})
			break
		}
	}
	seen := map[string]bool{}
	for _, node := range definition.Nodes {
		component := node.Executor + ":" + node.ConnectionVersion + ":" + node.Model
		if seen[component] {
			continue
		}
		seen[component] = true
		check := executorCheck(ctx, definition.ProjectID, node)
		out.Ready = out.Ready && check.Status != "unavailable"
		out.Checks = append(out.Checks, check)
	}
	return out
}

func executorCheck(ctx context.Context, project string, node workflow.Node) workflow.Check {
	check := workflow.Check{Component: node.Executor, Status: "configured"}
	native := node.Executor == "codex"
	if node.Executor == "eino" {
		chat, err := einoModel(ctx, workflow.NodeInput{ProjectID: project, Node: node})
		if err != nil || node.ConnectionVersion == "" && !einoConfigured() {
			check.Status = "unavailable"
			return check
		}
		_, native = chat.(*codexModel)
	}
	if !native {
		return check
	}
	check.Status = codexReady(ctx)
	if check.Status != "ready" || node.Model == "" {
		return check
	}
	models, err := CodexModels(ctx)
	if err == nil {
		for _, item := range models {
			if item.ID == node.Model {
				return check
			}
		}
	}
	return workflow.Check{Component: "codex_model", Status: "unavailable"}
}

func einoConfigured() bool {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		return false
	}
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" {
		return true
	}
	address, err := url.Parse(base)
	return err == nil && address.Hostname() != "" && address.User == nil && (address.Scheme == "https" || address.Scheme == "http")
}

func codexReady(ctx context.Context) string {
	binary, workspace, err := codexPaths(os.Getenv("PANGU_WORKSPACE_ROOT"))
	if err != nil {
		return "unavailable"
	}
	if checkCodexPolicy(ctx, binary) != nil {
		return "unavailable"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "login", "status")
	cmd.Stdin = strings.NewReader("")
	cmd.Dir = workspace
	cmd.Env, cmd.Stdout, cmd.Stderr = codexEnvironment(), io.Discard, io.Discard
	cmd.WaitDelay = time.Second
	if err := runCodexProcess(ctx, cmd, ""); err != nil {
		return "unavailable"
	}
	return "ready"
}
