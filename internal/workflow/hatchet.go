package workflow

import "fmt"

func Run(d Definition, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	return RunFrom(d, nil, runLayer)
}

func RunFrom(d Definition, reused map[string]NodeOutput, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	if err := d.ValidateInput(); err != nil {
		return nil, err
	}
	layers, err := d.Layers()
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]NodeOutput, len(d.Nodes))
	for id, out := range reused {
		outputs[id] = out
	}
	for _, layer := range layers {
		inputs := []NodeInput{}
		for _, n := range layer {
			if _, exists := outputs[n.ID]; exists {
				continue
			}
			input := d.NodeInput(n, outputs)
			selected, err := d.Selected(input)
			if err != nil {
				return nil, err
			}
			if !selected {
				outputs[n.ID] = NodeOutput{Skipped: true, Text: "条件未命中或直接上游已跳过"}
				continue
			}
			input, err = d.BindInput(input)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, input)
		}
		if len(inputs) == 0 {
			continue
		}
		results, err := runLayer(inputs)
		if err != nil {
			return nil, fmt.Errorf("执行节点层失败: %w", err)
		}
		if len(results) != len(inputs) {
			return nil, fmt.Errorf("节点结果数与提交数不一致")
		}
		for i, input := range inputs {
			n := input.Node
			if err := ValidateOutput(n, &results[i]); err != nil {
				return nil, err
			}
			outputs[n.ID] = results[i]
		}
	}
	return outputs, nil
}
