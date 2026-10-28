package workflow

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type ResearchBrief struct {
	Question      string   `json:"question"`
	Audience      string   `json:"audience"`
	Market        string   `json:"market"`
	TimeScope     string   `json:"time_scope"`
	Context       string   `json:"context"`
	Constraints   string   `json:"constraints"`
	PublicQueries []string `json:"public_queries,omitempty"`
	URLs          []string `json:"urls,omitempty"`
	Documents     []string `json:"documents,omitempty"`
	Competitors   []string `json:"competitors,omitempty"`
	Dimensions    []string `json:"dimensions,omitempty"`
}

func ParseBrief(input string, mode string) (ResearchBrief, error) {
	var b ResearchBrief
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&b); err != nil {
		return b, fmt.Errorf("产品经理输入必须符合 brief 结构: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return b, fmt.Errorf("产品经理输入含多余 JSON")
	}
	for _, v := range []string{b.Question, b.Audience, b.Market, b.TimeScope, b.Context, b.Constraints} {
		if strings.TrimSpace(v) == "" || len(v) > 16*1024 {
			return b, fmt.Errorf("问题、用户、市场、时间范围、现有资料和约束均必填，每项最多 16 KiB（未知请明确填写）")
		}
	}
	for _, list := range [][]string{b.PublicQueries, b.URLs, b.Documents, b.Competitors, b.Dimensions} {
		if len(list) > 10 {
			return b, fmt.Errorf("产品经理输入的每类列表最多 10 项")
		}
		for _, v := range list {
			if strings.TrimSpace(v) == "" || len(v) > 2048 {
				return b, fmt.Errorf("列表项必须为 1 至 2048 字节")
			}
		}
	}
	if mode == "competition" && (len(b.Competitors) == 0 || len(b.Dimensions) == 0) {
		return b, fmt.Errorf("竞品分析必须指定竞品候选和比较维度")
	}
	if mode == "research" && len(b.PublicQueries)+len(b.URLs)+len(b.Documents) == 0 {
		return b, fmt.Errorf("调研至少授权一个搜索词、网页或项目资料")
	}
	return b, nil
}

// Report 只由 Worker 工具产生；正文不进入 Hatchet 的节点输出。
type Report struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

func (n Node) validateRole() error {
	if n.ReadOnly && n.Executor != "codex" {
		return fmt.Errorf("read_only 只用于原生 Codex 沙箱；Eino 工具权限由 Worker 固定")
	}
	if n.Role == "" && n.Mode == "" {
		return nil
	}
	if n.Executor != "eino" || n.Role != "product_manager" || n.OutputSchema != nil {
		return fmt.Errorf("产品经理角色必须使用 Eino 和平台报告输出")
	}
	switch n.Mode {
	case "research", "competition", "planning", "design":
	default:
		return fmt.Errorf("产品经理模式必须为 research/competition/planning/design")
	}
	if n.EffectivePolicy().ToolCalls < 1 {
		return fmt.Errorf("产品经理节点至少需要一次报告登记工具调用")
	}
	return nil
}
