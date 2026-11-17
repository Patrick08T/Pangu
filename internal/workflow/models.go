package workflow

import "errors"

type ModelChoice struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ReasoningLevels  []string `json:"reasoning_levels,omitempty"`
	DefaultReasoning string   `json:"default_reasoning,omitempty"`
	Fast             bool     `json:"fast,omitempty"`
}

func ReasoningEffortValid(effort string) bool {
	switch effort {
	case "none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra":
		return true
	}
	return false
}

func (n Node) HasModelOptions() bool { return n.ReasoningEffort != "" || n.Fast != nil }

func (n Node) ValidateModelOptions() error {
	if !n.HasModelOptions() {
		return nil
	}
	if n.Model == "" {
		return errors.New("设置推理强度或 Fast 前必须选择具体模型")
	}
	if n.ReasoningEffort != "" && !ReasoningEffortValid(n.ReasoningEffort) {
		return errors.New("推理强度无效")
	}
	if n.Executor == "eino" && n.ConnectionVersion == "" {
		return errors.New("模型运行选项需要 Codex 或 Codex 账号连接")
	}
	return nil
}
