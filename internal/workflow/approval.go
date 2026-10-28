package workflow

import "fmt"

type Approval struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

func approvalLayers(layers [][]Node) error {
	for _, layer := range layers {
		approval, writer := false, false
		for _, node := range layer {
			approval = approval || node.Approval != nil
			writer = writer || node.Executor == "codex"
		}
		if len(layer) > 1 && approval && writer {
			return fmt.Errorf("节点 %s 所在层包含审批和 Codex 检查点变更，请通过 depends_on 排成单独的审批执行步骤", layer[0].ID)
		}
	}
	return nil
}
