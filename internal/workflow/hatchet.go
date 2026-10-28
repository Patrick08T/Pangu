package workflow

import "fmt"

func Run(d Definition, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	if err := d.ValidateInput(); err != nil {
		return nil, err
	}
	layers, err := d.Layers()
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]NodeOutput, len(d.Nodes))
	for _, layer := range layers {
		inputs := make([]NodeInput, len(layer))
		for i, n := range layer {
			inputs[i], err = d.BindInput(d.NodeInput(n, outputs))
			if err != nil {
				return nil, err
			}
		}
		results, err := runLayer(inputs)
		if err != nil {
			return nil, fmt.Errorf("执行节点层失败: %w", err)
		}
		if len(results) != len(layer) {
			return nil, fmt.Errorf("节点结果数与提交数不一致")
		}
		for i, n := range layer {
			if err := ValidateOutput(n, &results[i]); err != nil {
				return nil, err
			}
			outputs[n.ID] = results[i]
		}
	}
	return outputs, nil
}
