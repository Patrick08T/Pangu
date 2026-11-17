package factory

import (
	"strings"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (in Input) validateModel() error {
	if len(in.Model) > 128 || strings.TrimSpace(in.Model) != in.Model || strings.ContainsAny(in.Model, "\r\n\x00") {
		return invalid("模型名称无效")
	}
	node := workflow.Node{Executor: "eino", ConnectionVersion: in.ConnectionVersion, Model: in.Model, ReasoningEffort: in.ReasoningEffort, Fast: in.Fast}
	if err := node.ValidateModelOptions(); err != nil {
		return invalid(err.Error())
	}
	return nil
}

// 消息重试按参数值比较，不能用 Fast 指针地址比较 JSON 解码后的两次请求。
func (in Input) Equal(other Input) bool {
	left, right := in.Fast, other.Fast
	in.Fast, other.Fast = nil, nil
	return in == other && (left == nil && right == nil || left != nil && right != nil && *left == *right)
}

func configureModel(node *workflow.Node, in Input, choices []catalog.ConnectionChoice) error {
	for _, c := range choices {
		if c.ID != in.ConnectionVersion {
			continue
		}
		node.Model = c.Model
		if c.Provider != "codex" {
			if in.Model != "" && in.Model != c.Model || in.ReasoningEffort != "" || in.Fast != nil {
				return invalid("普通 API 连接使用固定模型，不支持 Codex 推理强度和 Fast")
			}
			return nil
		}
		if in.Model != "" {
			node.Model = in.Model
		}
		node.ReasoningEffort, node.Fast = in.ReasoningEffort, in.Fast
		return nil
	}
	return invalid("请选择当前可用的共享模型连接")
}
