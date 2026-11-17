package workflow

import (
	"errors"
	"fmt"
)

var ErrContinued = errors.New("已发布新版本并创建后续运行")

func Run(d Definition, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	return RunFrom(d, nil, runLayer)
}

func RunFrom(d Definition, reused map[string]NodeOutput, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	return RunControlled(d, reused, nil, runLayer)
}

func RunAudited(d Definition, reused map[string]NodeOutput, before func(*NodeInput, map[string]NodeOutput) error, after func(NodeInput, *NodeOutput) error, runLayer func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	if err := d.ValidateInput(); err != nil {
		return nil, err
	}
	if _, err := d.Layers(); err != nil {
		return nil, err
	}
	r := runner{definition: d, outputs: map[string]NodeOutput{}, before: before, after: after, execute: runLayer}
	for id, out := range reused {
		r.outputs[id] = out
	}
	layers, err := d.executionLayers()
	if err != nil {
		return nil, err
	}
	for _, layer := range layers {
		if err = r.runBlocks(layer); err != nil {
			if errors.Is(err, ErrContinued) {
				return r.outputs, nil
			}
			return nil, err
		}
	}
	return r.outputs, nil
}

type runner struct {
	definition Definition
	outputs    map[string]NodeOutput
	before     func(*NodeInput, map[string]NodeOutput) error
	after      func(NodeInput, *NodeOutput) error
	execute    func([]NodeInput) ([]NodeOutput, error)
}

func (r *runner) runBlocks(blocks []Node) error {
	nodes := []Node{}
	for _, block := range blocks {
		for _, loop := range r.definition.Loops {
			if block.ID == "@"+loop.ID {
				if err := r.runLoop(loop); err != nil {
					return err
				}
			}
		}
		for _, n := range r.definition.Nodes {
			if n.ID == block.ID {
				nodes = append(nodes, n)
			}
		}
	}
	return r.runLayer(nodes, nil)
}

func (r *runner) prepare(n Node, iteration *Iteration) (*NodeInput, error) {
	input := r.definition.NodeInput(n, r.outputs)
	input.Iteration = iteration
	selected, err := r.definition.Selected(input)
	if err != nil {
		return nil, err
	}
	if !selected {
		if iteration != nil {
			return nil, fmt.Errorf("循环成员 %s 的外部依赖被跳过，不能完成循环", n.ID)
		}
		return nil, r.finish(input, NodeOutput{Skipped: true, Text: "条件未命中或直接上游已跳过"})
	}
	input, err = r.definition.BindInput(input)
	if err != nil {
		return nil, err
	}
	if r.before != nil {
		if err = r.before(&input, r.outputs); err != nil {
			return nil, err
		}
	}
	if selected, err = r.definition.Selected(input); err != nil || !selected {
		return nil, fmt.Errorf("节点 %s 修订后不满足执行条件: %v", n.ID, err)
	}
	input, err = r.definition.BindInput(input)
	return &input, err
}

func (r *runner) runLayer(layer []Node, iteration *Iteration) error {
	inputs := []NodeInput{}
	for _, n := range layer {
		if _, ok := r.outputs[n.ID]; ok {
			continue
		}
		input, err := r.prepare(n, iteration)
		if err != nil {
			return err
		}
		if input != nil {
			inputs = append(inputs, *input)
		}
	}
	if len(inputs) == 0 {
		return nil
	}
	results, err := r.execute(inputs)
	if err != nil {
		return fmt.Errorf("执行节点层失败: %w", err)
	}
	if len(results) != len(inputs) {
		return fmt.Errorf("节点结果数与提交数不一致")
	}
	for i, input := range inputs {
		if err := r.finish(input, results[i]); err != nil {
			return err
		}
	}
	return nil
}
