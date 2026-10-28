package catalog

import (
	"math"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type FlowEditor struct {
	Nodes    map[string]NodeDisplay `json:"nodes,omitempty"`
	Viewport *Viewport              `json:"viewport,omitempty"`
}

type NodeDisplay struct {
	Name string  `json:"name,omitempty"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

var flowIdentifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func (flow Flow) validateDraft() error {
	if len(flow.Nodes) > 32 {
		return invalid("工作流最多允许 32 个节点")
	}
	if flow.Budget != nil {
		if err := flow.Budget.Validate(); err != nil {
			return err
		}
	}
	if flow.Parameters != nil {
		if flow.Parameters.Type != "object" {
			return invalid("流程输入必须为对象")
		}
		if _, err := flow.Parameters.Compile(); err != nil {
			return err
		}
	}
	ids := map[string]bool{}
	for _, node := range flow.Nodes {
		if err := node.validateStep(); err != nil {
			return err
		}
		if !flowIdentifier.MatchString(node.ID) || ids[node.ID] {
			return invalid("节点标识无效或重复")
		}
		ids[node.ID] = true
		if _, err := uuid.Parse(node.AgentVersion); err != nil {
			return invalid("工作流必须引用明确的 Agent 版本 ID")
		}
		if len(node.DependsOn) > 32 || len(node.Bindings) > 32 {
			return invalid("节点依赖或字段映射超过 32 项")
		}
	}
	if flow.Editor == nil {
		return nil
	}
	for id, display := range flow.Editor.Nodes {
		if !ids[id] || len(display.Name) > 128 || strings.ContainsRune(display.Name, '\x00') || !coordinate(display.X) || !coordinate(display.Y) {
			return invalid("画布节点信息无效：名称最多 128 字节，坐标范围为 ±1000000")
		}
	}
	if v := flow.Editor.Viewport; v != nil && (!coordinate(v.X) || !coordinate(v.Y) || math.IsNaN(v.Zoom) || v.Zoom < 0.1 || v.Zoom > 2) {
		return invalid("画布视图无效")
	}
	return nil
}

func coordinate(value float64) bool { return !math.IsNaN(value) && math.Abs(value) <= 1000000 }

func (node Binding) validateStep() error {
	if len(node.Task) > 8192 {
		return invalid("步骤任务最多 8192 字节")
	}
	if node.OutputSchema != nil {
		if node.OutputSchema.Type != "object" {
			return invalid("步骤输出必须为对象")
		}
		if _, err := node.OutputSchema.Compile(); err != nil {
			return err
		}
	}
	return nil
}
