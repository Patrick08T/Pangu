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
		if len(brief.PublicQueries) > 0 && os.Getenv("PANGU_SEARCH_API_KEY") == "" {
			out.Ready = false
			out.Checks = append(out.Checks, workflow.Check{Component: "search", Status: "unavailable"})
			break
		}
	}
	seen := map[string]bool{}
	for _, node := range definition.Nodes {
		component := node.Executor + ":" + node.ConnectionVersion
		if seen[component] {
			continue
		}
		seen[component] = true
		status := "configured"
		if node.Executor == "eino" {
			_, err := modelConfig(ctx, definition.ProjectID, node)
			if err != nil || (node.ConnectionVersion == "" && !einoConfigured()) {
				status = "unavailable"
			}
		}
		if node.Executor == "codex" {
			status = codexReady(ctx)
		}
		out.Ready = out.Ready && status != "unavailable"
		out.Checks = append(out.Checks, workflow.Check{Component: node.Executor, Status: status})
	}
	return out
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
