package workflow

import "fmt"

type Approval struct {
	ExecutionPlan bool     `json:"execution_plan,omitempty"`
	SourceNode    string   `json:"source_node"`
	RevisionNodes []string `json:"revision_nodes"`
}

func (d Definition) validateApprovals() error {
	nodes := map[string]Node{}
	for _, n := range d.Nodes {
		nodes[n.ID] = n
	}
	for _, n := range d.Nodes {
		if n.Approval == nil {
			continue
		}
		a := n.Approval
		if !containsID(n.DependsOn, a.SourceNode) || len(a.RevisionNodes) == 0 {
			return fmt.Errorf("节点 %s 审批必须指定直接上游 source_node 和只读 revision_nodes", n.ID)
		}
		ancestors := map[string]bool{}
		var visit func(string)
		visit = func(id string) {
			if ancestors[id] {
				return
			}
			ancestors[id] = true
			for _, parent := range nodes[id].DependsOn {
				visit(parent)
			}
		}
		for _, id := range n.DependsOn {
			visit(id)
		}
		seen := map[string]bool{}
		for _, id := range a.RevisionNodes {
			r, ok := nodes[id]
			if !ok || !ancestors[id] || seen[id] || r.Executor != "codex" || !r.ReadOnly || r.Role != "" || r.Approval != nil {
				return fmt.Errorf("节点 %s 的修订节点 %s 必须是无审批的只读 Codex 上游", n.ID, id)
			}
			seen[id] = true
		}
		if !seen[a.SourceNode] {
			return fmt.Errorf("审核来源 %s 必须参与最终修订汇总", a.SourceNode)
		}
		// 修订输出不能在批准前被未修订的已完成节点继续使用。
		for _, r := range d.Nodes {
			for _, parent := range r.DependsOn {
				if ancestors[r.ID] && seen[parent] && !seen[r.ID] {
					return fmt.Errorf("修订集合遗漏受影响上游 %s", r.ID)
				}
			}
		}
	}
	return nil
}

func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func approvalLayers(layers [][]Node) error {
	completed := []Node{}
	for _, layer := range layers {
		for _, node := range layer {
			if node.Approval == nil {
				continue
			}
			if len(layer) > 1 {
				return fmt.Errorf("审批节点 %s 必须独占执行层，避免其他节点提前构造旧计划输入", node.ID)
			}
			for _, prior := range completed {
				for _, parent := range prior.DependsOn {
					if containsID(node.Approval.RevisionNodes, parent) && !containsID(node.Approval.RevisionNodes, prior.ID) {
						return fmt.Errorf("审批节点 %s 的修订集合遗漏已完成的受影响节点 %s", node.ID, prior.ID)
					}
				}
			}
		}
		completed = append(completed, layer...)
	}
	return nil
}
