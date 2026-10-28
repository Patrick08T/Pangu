package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

type codexSearch struct {
	Query     string
	raw       json.RawMessage
	err       error
	completed bool
}

func (s *codexSearch) configure(cmd *exec.Cmd) {
	cmd.Args = append(cmd.Args[:len(cmd.Args)-1], "-c", `web_search="live"`, "--enable", "standalone_web_search", "-")
	cmd.Env = append(cmd.Env, "PANGU_CODEX_SEARCH_QUERY="+s.Query)
}

func codexPublicSearch(ctx context.Context, input workflow.NodeInput, query string) ([]research.SearchResult, error) {
	model := ""
	if input.Node.ConnectionVersion != "" {
		connection, _, err := catalog.ResolveConnection(ctx, input.ProjectID, input.Node.ConnectionVersion)
		if err != nil {
			return nil, err
		}
		if connection.Provider == "codex" {
			model = connection.Model
		}
	}
	root := os.Getenv("PANGU_WORKSPACE_ROOT")
	if !filepath.IsAbs(root) {
		return nil, errors.New("Codex 搜索需要 Worker 私有工作区绝对路径")
	}
	dir, err := os.MkdirTemp(root, "search-codex-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "response.json")
	if err := os.WriteFile(path, []byte(`{"type":"object","properties":{"done":{"type":"boolean"}},"required":["done"],"additionalProperties":false}`), 0600); err != nil {
		return nil, err
	}
	input = workflow.NodeInput{Workspace: dir, ProcessScope: input.ProcessScope, Input: query, Node: workflow.Node{ID: "search", Executor: "codex", Model: model, ReadOnly: true,
		Instruction: "只执行一次 web.run 公开搜索，search_query 的 q 为 input 中的原始搜索词。平台固定搜索参数，不允许改词、打开页面、其他工具或本机文件读写。搜索完成后直接返回 done=true；不需要整理、推断或生成来源。",
		Policy:      &workflow.Policy{}}}
	search := &codexSearch{Query: query}
	_, err = runCodex(ctx, input, path, search)
	if err != nil {
		return nil, err
	}
	return search.results()
}

func (s *codexSearch) record(line []byte) {
	var event struct {
		Type string
		Item struct {
			Type   string
			Action struct {
				Type, Query string
				Queries     []string
			}
			Results json.RawMessage
		}
	}
	if json.Unmarshal(line, &event) != nil || event.Type != "item.completed" || event.Item.Type != "web_search" {
		return
	}
	if s.completed || event.Item.Action.Type != "search" || event.Item.Action.Query != s.Query || len(event.Item.Action.Queries) != 0 {
		s.err = errors.New("Codex 搜索返回了非授权查询或多次搜索")
		return
	}
	s.completed = true
	s.raw = event.Item.Results
}

func (s *codexSearch) results() ([]research.SearchResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	if !s.completed {
		return nil, errors.New("Codex 未完成原生联网搜索；不能以模型回答代替搜索结果")
	}
	var items []research.SearchResult
	if json.Unmarshal(s.raw, &items) != nil || items == nil || len(items) > 100 {
		return nil, errors.New("Codex 未返回可验证的结构化搜索结果")
	}
	return items, nil
}
