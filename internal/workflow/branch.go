package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
)

type Condition struct {
	Field  FieldBinding    `json:"field"`
	Equals json.RawMessage `json:"equals"`
}

func (d Definition) validateConditions() error {
	for _, node := range d.Nodes {
		if node.When == nil {
			continue
		}
		condition := node.When
		var value any
		if len(condition.Equals) > 1024 || json.Unmarshal(condition.Equals, &value) != nil || (condition.Field.Type != "boolean" && condition.Field.Type != "string") || !fieldType(condition.Field.Type, value) {
			return fmt.Errorf("节点 %s 条件仅支持同类型布尔值或字符串精确匹配", node.ID)
		}
		node.Bindings = map[string]FieldBinding{"condition": condition.Field}
		if err := (Definition{InputSchema: d.InputSchema, Nodes: []Node{node}}).validateBindings(); err != nil {
			return err
		}
	}
	return nil
}

func (d Definition) Selected(input NodeInput) (bool, error) {
	for _, parent := range input.Parents {
		if parent.Skipped {
			return false, nil
		}
	}
	condition := input.Node.When
	if condition == nil {
		return true, nil
	}
	input.Node.Bindings = map[string]FieldBinding{"condition": condition.Field}
	bound, err := d.BindInput(input)
	if err != nil {
		return false, err
	}
	var expected any
	if err = json.Unmarshal(condition.Equals, &expected); err != nil {
		return false, err
	}
	return reflect.DeepEqual(bound.Parameters["condition"], expected), nil
}

type InputPreview struct {
	NodeID     string         `json:"node_id"`
	State      string         `json:"state"`
	Parameters map[string]any `json:"parameters"`
	Error      string         `json:"error"`
}

func PreviewInputs(d Definition, outputs map[string]NodeOutput) ([]InputPreview, error) {
	layers, err := d.Layers()
	if err != nil {
		return nil, err
	}
	if err := d.ValidateInput(); err != nil {
		return nil, err
	}
	out := []InputPreview{}
	known := maps.Clone(outputs)
	if known == nil {
		known = map[string]NodeOutput{}
	}
	for _, layer := range layers {
		for _, node := range layer {
			item := InputPreview{NodeID: node.ID, State: "ready", Parameters: map[string]any{}}
			missing := false
			for _, id := range node.DependsOn {
				if _, ok := known[id]; !ok {
					missing = true
				}
			}
			if missing {
				item.State = "pending"
				out = append(out, item)
				continue
			}
			input := d.NodeInput(node, known)
			selected, err := d.Selected(input)
			if err == nil && !selected {
				item.State = "skipped"
				known[node.ID] = NodeOutput{Skipped: true}
			} else if err == nil {
				input, err = d.BindInput(input)
				item.Parameters = input.Parameters
			}
			if err != nil {
				item.State, item.Error = "invalid", err.Error()
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func Prefix(d, previous Definition, checkpoint string) ([]string, error) {
	layers, err := previous.Layers()
	if err != nil {
		return nil, err
	}
	current, err := d.Layers()
	if err != nil {
		return nil, err
	}
	ids := []string{}
	same := d.Input == previous.Input
	for i, layer := range layers {
		if len(layer) != 1 || layer[0].Executor != "codex" || layer[0].When != nil {
			return nil, errors.New("检查点返工仅支持串行、无条件 Codex 前缀")
		}
		same = same && i < len(current) && len(current[i]) == 1 && equalNode(layer[0], current[i][0])
		if same {
			ids = append(ids, layer[0].ID)
		}
		if layer[0].ID == checkpoint {
			return ids, nil
		}
	}
	return nil, errors.New("来源检查点节点不存在")
}

func equalNode(a, b Node) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
