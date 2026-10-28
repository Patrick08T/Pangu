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
	if strings.TrimSpace(b.Question) == "" {
		return b, fmt.Errorf("请填写任务目标")
	}
	for _, v := range []string{b.Question, b.Audience, b.Market, b.TimeScope, b.Context, b.Constraints} {
		if len(v) > 16*1024 {
			return b, fmt.Errorf("任务目标和每项背景最多 16 KiB")
		}
	}
	for _, list := range []struct {
		name   string
		values []string
	}{
		{"允许公开发送的搜索词", b.PublicQueries}, {"授权公开网址", b.URLs},
		{"授权项目相对文件", b.Documents}, {"竞品候选", b.Competitors}, {"比较维度", b.Dimensions},
	} {
		if len(list.values) > 10 {
			return b, fmt.Errorf("%s最多 10 项", list.name)
		}
		for i, v := range list.values {
			if strings.TrimSpace(v) == "" {
				return b, fmt.Errorf("%s第 %d 项不能为空，请填写或删除该项", list.name, i+1)
			}
			if len(v) > 2048 {
				return b, fmt.Errorf("%s第 %d 项不能超过 2048 字节", list.name, i+1)
			}
		}
	}
	if mode == "competition" && (len(b.Competitors) == 0 || len(b.Dimensions) == 0) {
		return b, fmt.Errorf("竞品分析必须指定竞品候选和比较维度")
	}
	if (mode == "research" || mode == "competition") && len(b.PublicQueries)+len(b.URLs)+len(b.Documents) == 0 {
		return b, fmt.Errorf("调研和竞品分析至少提供一个公开搜索词、授权网址或项目资料；仅填写竞品名称不会开启联网搜索")
	}
	return b, nil
}

// Report 只由 Worker 工具产生；正文不进入 Hatchet 的节点输出。
type Report struct {
	Metadata json.RawMessage `json:"-"`
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Markdown string          `json:"markdown"`
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
	return nil
}
