package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Loop 将无环子图作为一个执行块；只有本轮全部质量门通过才能离开。
type Loop struct {
	ID    string   `json:"id"`
	Nodes []string `json:"nodes"`
	Gates []string `json:"gates"`
}

type Iteration struct {
	ID       string        `json:"id"`
	Round    int           `json:"round"`
	Gate     bool          `json:"gate,omitempty"`
	Feedback *LoopFeedback `json:"feedback,omitempty"`
}

type LoopFeedback struct {
	Round     int      `json:"round"`
	NodeID    string   `json:"node_id"`
	NodeRunID string   `json:"node_run_id"`
	Summary   string   `json:"summary"`
	ReportIDs []string `json:"report_ids,omitempty"`
}

func LoopGateSchema() *Shape {
	return &Shape{Type: "object", Properties: map[string]Shape{"passed": {Type: "boolean"}, "summary": {Type: "string"}}, Required: []string{"passed", "summary"}}
}

func GateVerdict(out NodeOutput) (bool, string, error) {
	var value struct {
		Passed  *bool  `json:"passed"`
		Summary string `json:"summary"`
	}
	if out.Skipped || len(out.Blockers) > 0 || json.Unmarshal([]byte(out.Text), &value) != nil || value.Passed == nil || strings.TrimSpace(value.Summary) == "" {
		return false, "", fmt.Errorf("循环质量门必须返回明确的 passed 和非空 summary，不能跳过")
	}
	return *value.Passed, value.Summary, nil
}

func (d Definition) LoopFor(node string) *Loop {
	for i := range d.Loops {
		if containsID(d.Loops[i].Nodes, node) {
			return &d.Loops[i]
		}
	}
	return nil
}

func (d Definition) validateExecution(layers [][]Node) error {
	if err := approvalLayers(layers); err != nil {
		return err
	}
	seen, nodes := map[string]bool{}, map[string]Node{}
	for _, n := range d.Nodes {
		nodes[n.ID] = n
	}
	members := map[string]string{}
	for _, loop := range d.Loops {
		if !identifier.MatchString(loop.ID) || seen[loop.ID] || len(loop.Nodes) < 2 || len(loop.Nodes) > 32 || len(loop.Gates) == 0 || len(loop.Gates) > len(loop.Nodes) {
			return fmt.Errorf("循环块标识、成员或质量门无效")
		}
		seen[loop.ID] = true
		for _, id := range loop.Nodes {
			n, ok := nodes[id]
			if !ok || members[id] != "" || n.Executor != "codex" || n.Role != "" || n.When != nil || n.Approval != nil {
				return fmt.Errorf("循环 %s 成员 %s 无效：需不重叠、无条件和审批的 Codex 节点", loop.ID, id)
			}
			members[id] = loop.ID
		}
		if err := loop.validateGates(nodes); err != nil {
			return err
		}
	}
	for _, n := range d.Nodes {
		for _, parent := range n.DependsOn {
			if loop := d.LoopFor(parent); loop != nil && members[n.ID] != loop.ID && parent != loop.Gates[len(loop.Gates)-1] {
				return fmt.Errorf("循环 %s 仅允许从最后质量门连接块外节点", loop.ID)
			}
		}
		if n.Approval != nil {
			for _, id := range n.Approval.RevisionNodes {
				if members[id] != "" {
					return fmt.Errorf("人工修订不得改写循环块内结果")
				}
			}
		}
	}
	_, err := d.executionLayers()
	return err
}

func (loop Loop) validateGates(nodes map[string]Node) error {
	seen := map[string]bool{}
	for i, id := range loop.Gates {
		n := nodes[id]
		if !containsID(loop.Nodes, id) || seen[id] || !n.ReadOnly || n.OutputSchema == nil {
			return fmt.Errorf("循环 %s 质量门必须是独立只读成员并声明输出结构", loop.ID)
		}
		for name, kind := range map[string]string{"passed": "boolean", "summary": "string"} {
			if n.OutputSchema.Properties[name].Type != kind || !containsID(n.OutputSchema.Required, name) {
				return fmt.Errorf("质量门 %s 缺少必填字段 %s", id, name)
			}
		}
		if i > 0 && !nodeAncestor(nodes, loop.Gates[i-1], id, map[string]bool{}) {
			return fmt.Errorf("循环质量门必须按声明顺序依次执行")
		}
		seen[id] = true
	}
	for _, id := range loop.Nodes {
		if !seen[id] && !nodeAncestor(nodes, id, loop.Gates[0], map[string]bool{}) {
			return fmt.Errorf("循环 %s 所有修改步骤必须在首个质量门之前完成", loop.ID)
		}
	}
	return nil
}

func nodeAncestor(nodes map[string]Node, ancestor, node string, seen map[string]bool) bool {
	if seen[node] {
		return false
	}
	seen[node] = true
	for _, id := range nodes[node].DependsOn {
		if id == ancestor || nodeAncestor(nodes, ancestor, id, seen) {
			return true
		}
	}
	return false
}

// 折叠循环后再次检查依赖，避免块外路径绕回块内造成死锁。
func (d Definition) executionLayers() ([][]Node, error) {
	group := map[string]string{}
	for _, n := range d.Nodes {
		group[n.ID] = n.ID
	}
	for _, loop := range d.Loops {
		for _, id := range loop.Nodes {
			group[id] = "@" + loop.ID
		}
	}
	indices := map[string]int{}
	flat := Definition{}
	for _, n := range d.Nodes {
		id := group[n.ID]
		i, ok := indices[id]
		if !ok {
			i = len(flat.Nodes)
			indices[id] = i
			flat.Nodes = append(flat.Nodes, Node{ID: id})
		}
		for _, p := range n.DependsOn {
			if g := group[p]; g != id && !containsID(flat.Nodes[i].DependsOn, g) {
				flat.Nodes[i].DependsOn = append(flat.Nodes[i].DependsOn, g)
			}
		}
	}
	return flat.layers()
}
