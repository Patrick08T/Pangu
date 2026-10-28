package workflow

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type HandoffContract struct {
	Objective       string `json:"objective"`
	Inputs          string `json:"inputs"`
	Deliverables    string `json:"deliverables"`
	Acceptance      string `json:"acceptance"`
	FailureHandling string `json:"failure_handling"`
}

func (h *HandoffContract) fields() [][2]string {
	if h == nil {
		h = &HandoffContract{}
	}
	return [][2]string{{"任务目标", h.Objective}, {"输入与上下文", h.Inputs}, {"交付物", h.Deliverables}, {"验收标准", h.Acceptance}, {"异常处理", h.FailureHandling}}
}

func (h *HandoffContract) Validate(required bool) error {
	for _, field := range h.fields() {
		if required && strings.TrimSpace(field[1]) == "" {
			return fmt.Errorf("交接契约的%s必填；请补齐草稿，已有发布版本需重新发布", field[0])
		}
		if !utf8.ValidString(field[1]) || len(field[1]) > 4096 {
			return fmt.Errorf("交接契约的%s必须是 UTF-8 文本且不超过 4096 字节", field[0])
		}
	}
	return nil
}

func (h *HandoffContract) Instruction() string {
	if h == nil {
		return ""
	}
	var text strings.Builder
	text.WriteString("\n交接契约（不得扩大本次资料授权、工具权限或预算；未满足验收标准时明确说明缺口，不得声称完成）：")
	for _, field := range h.fields() {
		fmt.Fprintf(&text, "\n%s：%s", field[0], field[1])
	}
	return text.String()
}

func (d Definition) ValidateHandoffs() error {
	for _, node := range d.Nodes {
		if err := node.Handoff.Validate(true); err != nil {
			return fmt.Errorf("节点 %s：%w", node.ID, err)
		}
	}
	return nil
}
