package workflow

import (
	"fmt"
	"unicode/utf8"
)

func (r *runner) runLoop(loop Loop) error {
	if r.skipLoop(loop) {
		return nil
	}
	layers, err := r.loopLayers(loop)
	if err != nil {
		return err
	}
	iteration, err := r.resumeIteration(loop)
	if err != nil {
		return err
	}
	for {
		feedback, err := r.loopRound(loop, layers, iteration)
		if err != nil {
			return err
		}
		if feedback == nil {
			return nil
		}
		for _, id := range loop.Nodes {
			delete(r.outputs, id)
		}
		iteration = &Iteration{ID: loop.ID, Round: iteration.Round + 1, Feedback: feedback}
	}
}

func (r *runner) skipLoop(loop Loop) bool {
	for _, n := range r.definition.Nodes {
		if !containsID(loop.Nodes, n.ID) {
			continue
		}
		for _, parent := range n.DependsOn {
			if !containsID(loop.Nodes, parent) && r.outputs[parent].Skipped {
				// 循环是原子执行块，先核对全部外部依赖，避免修改后才发现后续质量门不能执行。
				for _, id := range loop.Nodes {
					r.outputs[id] = NodeOutput{Skipped: true, Text: "循环块外部依赖已跳过"}
				}
				return true
			}
		}
	}
	return false
}

func (r *runner) loopLayers(loop Loop) ([][]Node, error) {
	d := Definition{}
	for _, n := range r.definition.Nodes {
		if !containsID(loop.Nodes, n.ID) {
			continue
		}
		parents := n.DependsOn
		n.DependsOn = nil
		for _, parent := range parents {
			if containsID(loop.Nodes, parent) {
				n.DependsOn = append(n.DependsOn, parent)
			}
		}
		d.Nodes = append(d.Nodes, n)
	}
	layers, err := d.layers()
	for i := range layers {
		for j := range layers[i] {
			layers[i][j] = findNode(r.definition, layers[i][j].ID)
		}
	}
	return layers, err
}

func findNode(d Definition, id string) Node {
	for _, n := range d.Nodes {
		if n.ID == id {
			return n
		}
	}
	return Node{}
}

func (r *runner) resumeIteration(loop Loop) (*Iteration, error) {
	current := &Iteration{ID: loop.ID, Round: 1}
	found := false
	for _, id := range loop.Nodes {
		out, ok := r.outputs[id]
		if !ok {
			continue
		}
		it := out.Iteration
		if it == nil || it.ID != loop.ID || it.Round < 1 || found && it.Round != current.Round {
			return nil, fmt.Errorf("循环 %s 的复用成果缺少轮次或混合了不同轮次", loop.ID)
		}
		copy := *it
		current = &copy
		found = true
	}
	return current, nil
}

func (r *runner) loopRound(loop Loop, layers [][]Node, iteration *Iteration) (*LoopFeedback, error) {
	revision := ""
	for _, layer := range layers {
		copy := *iteration
		copy.Gate = containsID(loop.Gates, layer[0].ID)
		if err := r.runLayer(layer, &copy); err != nil {
			return nil, err
		}
		for _, n := range layer {
			if !containsID(loop.Gates, n.ID) {
				continue
			}
			out := r.outputs[n.ID]
			if out.ContentRevision == "" || revision != "" && revision != out.ContentRevision {
				return nil, fmt.Errorf("循环 %s 质量检查缺少代码摘要或验收期间代码发生变化", loop.ID)
			}
			revision = out.ContentRevision
			passed, summary, err := GateVerdict(out)
			if err != nil {
				return nil, err
			}
			if !passed {
				return &LoopFeedback{Round: iteration.Round, NodeID: n.ID, NodeRunID: out.RunID, Summary: feedbackSummary(summary), ReportIDs: out.ReportIDs}, nil
			}
		}
	}
	return nil, nil
}

func feedbackSummary(value string) string {
	if len(value) <= 2048 {
		return value
	}
	value = value[:2048]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + "（摘要已截断，完整证据见节点运行）"
}
