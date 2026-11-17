package workflow

import "encoding/json"

type Outcome struct {
	State string        `json:"state"`
	Nodes []OutcomeNode `json:"nodes"`
}

type OutcomeNode struct {
	NodeID  string `json:"node_id"`
	State   string `json:"state"`
	Summary string `json:"summary,omitempty"`
}

// BusinessOutcome 只读取终端节点的结构化验收结论，执行成功不是业务通过。
func BusinessOutcome(d Definition, outputs map[string]NodeOutput, execution string) Outcome {
	parents := map[string]bool{}
	for _, node := range d.Nodes {
		for _, id := range node.DependsOn {
			parents[id] = true
		}
	}
	out := Outcome{State: "not_assessed", Nodes: []OutcomeNode{}}
	accepted, attention := 0, false
	for _, node := range d.Nodes {
		if parents[node.ID] {
			continue
		}
		result := terminalOutcome(node, outputs[node.ID])
		out.Nodes = append(out.Nodes, result)
		if result.State == "accepted" {
			accepted++
		}
		attention = attention || result.State == "needs_attention"
	}
	if execution != "succeeded" && execution != "failed" && execution != "cancelled" {
		out.State = "pending"
	} else if attention || (accepted > 0 && execution != "succeeded") {
		out.State = "needs_attention"
	} else if accepted > 0 && accepted == len(out.Nodes) {
		out.State = "accepted"
	}
	return out
}

func terminalOutcome(node Node, output NodeOutput) OutcomeNode {
	out := OutcomeNode{NodeID: node.ID, State: "not_assessed"}
	if output.Iteration != nil && output.Iteration.Gate {
		passed, summary, err := GateVerdict(output)
		out.Summary, out.State = summary, "needs_attention"
		if err == nil && passed && output.ContentRevision != "" {
			out.State = "accepted"
		}
		return out
	}
	var result struct {
		Complete *bool  `json:"iteration_complete"`
		Ready    bool   `json:"ready"`
		Status   string `json:"status"`
		Summary  string `json:"summary"`
	}
	if output.Skipped || json.Unmarshal([]byte(output.Text), &result) != nil || result.Complete == nil {
		return out
	}
	out.Summary, out.State = result.Summary, "needs_attention"
	if *result.Complete && result.Ready && result.Status == "complete" {
		out.State = "accepted"
	}
	return out
}
