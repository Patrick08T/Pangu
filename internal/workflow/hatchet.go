package workflow

import "fmt"

func Run(d Definition, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	layers, err := d.Layers()
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]NodeOutput, len(d.Nodes))
	for _, layer := range layers {
		inputs := make([]NodeInput, len(layer))
		for i, n := range layer {
			inputs[i] = d.NodeInput(n, outputs)
		}
		results, err := runLayer(inputs)
		if err != nil {
			return nil, fmt.Errorf("执行节点层失败: %w", err)
		}
		if len(results) != len(layer) {
			return nil, fmt.Errorf("节点结果数与提交数不一致")
		}
		for i, n := range layer {
			outputs[n.ID] = results[i]
		}
	}
	return outputs, nil
}
