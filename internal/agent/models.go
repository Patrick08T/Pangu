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
	"slices"
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

type bundledModel struct {
	Slug             string `json:"slug"`
	Name             string `json:"display_name"`
	Visibility       string `json:"visibility"`
	Priority         int    `json:"priority"`
	DefaultReasoning string `json:"default_reasoning_level"`
	Reasoning        []struct {
		Effort string `json:"effort"`
	} `json:"supported_reasoning_levels"`
	Tiers []struct {
		ID string `json:"id"`
	} `json:"service_tiers"`
}

func codexModelChoices(raw []byte) ([]workflow.ModelChoice, error) {
	var catalog struct {
		Models []bundledModel `json:"models"`
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
		item, err := modelCapabilities(model, name)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func modelCapabilities(model bundledModel, name string) (workflow.ModelChoice, error) {
	item := workflow.ModelChoice{ID: model.Slug, Name: name, DefaultReasoning: model.DefaultReasoning}
	for _, level := range model.Reasoning {
		if !workflow.ReasoningEffortValid(level.Effort) {
			return item, errors.New("模型目录中的推理强度无效")
		}
		if !slices.Contains(item.ReasoningLevels, level.Effort) {
			item.ReasoningLevels = append(item.ReasoningLevels, level.Effort)
		}
	}
	if item.DefaultReasoning != "" && !slices.Contains(item.ReasoningLevels, item.DefaultReasoning) {
		return item, errors.New("模型默认推理强度不在支持列表中")
	}
	for _, tier := range model.Tiers {
		item.Fast = item.Fast || tier.ID == "priority"
	}
	return item, nil
}
