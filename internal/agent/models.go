package agent

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// 每次从实际执行程序读取原始内置目录，不读取 Codex 缓存或调用模型。
func CodexModels(ctx context.Context) ([]workflow.ModelChoice, error) {
	binary, workspace, err := codexPaths(os.Getenv("PANGU_WORKSPACE_ROOT"))
	if err != nil {
		return nil, errors.New("Worker 的 Codex 程序尚未就绪")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "debug", "models", "--bundled")
	output := diagnostic.Buffer{Limit: 4 * 1024 * 1024}
	cmd.Dir, cmd.Env, cmd.Stdin = workspace, codexEnvironment(), strings.NewReader("")
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = &output, io.Discard, time.Second
	if err = runCodexProcess(ctx, cmd, ""); err != nil || output.Truncated {
		return nil, errors.New("无法读取 Worker 的 Codex 模型目录")
	}
	return codexModelChoices([]byte(output.Data.String()))
}

func codexModelChoices(raw []byte) ([]workflow.ModelChoice, error) {
	var catalog struct {
		Models []struct {
			Slug       string `json:"slug"`
			Name       string `json:"display_name"`
			Visibility string `json:"visibility"`
			Priority   int    `json:"priority"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil || catalog.Models == nil || len(catalog.Models) > 512 {
		return nil, errors.New("Codex 模型目录格式无效")
	}
	sort.SliceStable(catalog.Models, func(i, j int) bool { return catalog.Models[i].Priority < catalog.Models[j].Priority })
	items := []workflow.ModelChoice{}
	seen := map[string]bool{}
	validID := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
	for _, model := range catalog.Models {
		if model.Visibility != "list" {
			continue
		}
		if !validID.MatchString(model.Slug) {
			return nil, errors.New("Codex 模型标识无效")
		}
		if seen[model.Slug] {
			continue
		}
		seen[model.Slug] = true
		name := strings.TrimSpace(model.Name)
		if name == "" {
			name = model.Slug
		}
		if len(name) > 256 || strings.ContainsAny(name, "\r\n\x00") {
			return nil, errors.New("Codex 模型名称无效")
		}
		items = append(items, workflow.ModelChoice{ID: model.Slug, Name: name})
	}
	return items, nil
}
