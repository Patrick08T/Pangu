package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Shape 是无引用、无脚本的 JSON Schema 子集，所有对象禁止未知字段。
type Shape struct {
	Type       string           `json:"type"`
	Properties map[string]Shape `json:"properties,omitempty"`
	Required   []string         `json:"required,omitempty"`
	Items      *Shape           `json:"items,omitempty"`
}

type FieldBinding struct {
	From    string `json:"from"`
	Pointer string `json:"pointer"`
	Type    string `json:"type"`
}

func (s *Shape) Compile() (*jsonschema.Schema, error) {
	doc, err := s.document(0)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(doc)
	if err != nil || len(data) > 16384 {
		return nil, errors.New("结构定义超过 16 KiB")
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource("urn:pangu:schema", doc); err != nil {
		return nil, err
	}
	return c.Compile("urn:pangu:schema")
}

func (s *Shape) document(depth int) (map[string]any, error) {
	if s == nil || depth > 6 || !shapeType(s.Type) {
		return nil, errors.New("结构类型无效或嵌套超过 6 层")
	}
	doc := map[string]any{"type": s.Type}
	if s.Type != "object" && (len(s.Properties) > 0 || len(s.Required) > 0) || s.Type != "array" && s.Items != nil {
		return nil, errors.New("结构属性与类型不匹配")
	}
	if s.Type == "object" {
		if len(s.Properties) > 64 {
			return nil, errors.New("单层结构字段超过 64 个")
		}
		properties := map[string]any{}
		for name, property := range s.Properties {
			if !identifier.MatchString(name) {
				return nil, errors.New("结构字段名称无效")
			}
			value, err := property.document(depth + 1)
			if err != nil {
				return nil, err
			}
			properties[name] = value
		}
		required := []any{}
		seen := map[string]bool{}
		for _, name := range s.Required {
			if _, exists := s.Properties[name]; !exists || seen[name] {
				return nil, errors.New("必填字段不存在或重复")
			}
			required = append(required, name)
			seen[name] = true
		}
		doc["properties"], doc["required"], doc["additionalProperties"] = properties, required, false
	}
	if s.Type == "array" {
		value, err := s.Items.document(depth + 1)
		if err != nil {
			return nil, err
		}
		doc["items"] = value
	}
	return doc, nil
}

func shapeType(value string) bool {
	switch value {
	case "string", "number", "integer", "boolean", "object", "array", "null":
		return true
	}
	return false
}

func (d Definition) validateBindings() error {
	if d.InputSchema != nil {
		if d.InputSchema.Type != "object" {
			return errors.New("工作流参数根类型必须为 object")
		}
		if _, err := d.InputSchema.Compile(); err != nil {
			return err
		}
	}
	for _, node := range d.Nodes {
		if len(node.Bindings) > 32 {
			return errors.New("单节点字段绑定超过 32 个")
		}
		for name, binding := range node.Bindings {
			if !identifier.MatchString(name) || !shapeType(binding.Type) || len(binding.Pointer) > 256 || binding.Pointer != "" && !strings.HasPrefix(binding.Pointer, "/") {
				return fmt.Errorf("节点 %s 参数 %s 的绑定无效", node.ID, name)
			}
			if binding.From == "$input" {
				if d.InputSchema == nil {
					return errors.New("工作流输入绑定需要声明 input_schema")
				}
				continue
			}
			found := false
			for _, parent := range node.DependsOn {
				found = found || parent == binding.From
			}
			if !found {
				return fmt.Errorf("节点 %s 只能绑定直接上游字段", node.ID)
			}
		}
	}
	return nil
}

func (s *Shape) Validate(value any) error {
	if err := transportNumbers(value); err != nil {
		return err
	}
	compiled, err := s.Compile()
	if err != nil {
		return err
	}
	return compiled.Validate(value)
}

func ValidateOutput(node Node, output *NodeOutput) error {
	if output.Text == "" || len(output.Text) > 16384 {
		return fmt.Errorf("节点 %s 输出为空或超过 16 KiB", node.ID)
	}
	if node.OutputSchema != nil {
		value, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(output.Text))
		if err != nil {
			return fmt.Errorf("节点 %s 结构化输出必须为 JSON: %w", node.ID, err)
		}
		if err := node.OutputSchema.Validate(value); err != nil {
			return fmt.Errorf("节点 %s 输出结构错误: %w", node.ID, err)
		}
		output.Data, _ = value.(map[string]any)
	}
	final := *output
	if final.RunID == "" {
		final.RunID = "00000000-0000-0000-0000-000000000000"
	}
	data, err := json.Marshal(final)
	if err != nil || len(data) > 16384 {
		return fmt.Errorf("节点 %s 序列化输出超过 16 KiB", node.ID)
	}
	return nil
}

func (d Definition) ValidateInput() error {
	for _, n := range d.Nodes {
		if n.Role != "" {
			if _, err := ParseBrief(d.Input, n.Mode); err != nil {
				return err
			}
		}
	}
	if d.InputSchema == nil {
		return nil
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(d.Input))
	if err != nil {
		return fmt.Errorf("工作流输入必须为 JSON: %w", err)
	}
	return d.InputSchema.Validate(value)
}

func (d Definition) BindInput(input NodeInput) (NodeInput, error) {
	input.Parameters = map[string]any{}
	for name, binding := range input.Node.Bindings {
		var source any
		if binding.From == "$input" {
			var err error
			source, err = jsonschema.UnmarshalJSON(bytes.NewBufferString(d.Input))
			if err != nil {
				return input, fmt.Errorf("参数 %s 需要 JSON 工作流输入", name)
			}
		} else {
			parent, exists := input.Parents[binding.From]
			if !exists {
				return input, fmt.Errorf("参数 %s 的直接上游不存在", name)
			}
			source = parent.Data
		}
		value, err := selectField(source, binding.Pointer)
		if err != nil || !fieldType(binding.Type, value) {
			return input, fmt.Errorf("节点 %s 参数 %s 路径不存在或类型不符", input.Node.ID, name)
		}
		input.Parameters[name] = value
	}
	return input, nil
}

func selectField(value any, pointer string) (any, error) {
	if pointer == "" {
		return value, nil
	}
	if !strings.HasPrefix(pointer, "/") || len(pointer) > 256 {
		return nil, errors.New("字段路径必须为不超过 256 字节的 JSON Pointer")
	}
	for _, token := range strings.Split(pointer[1:], "/") {
		for i := 0; i < len(token); i++ {
			if token[i] == '~' {
				i++
				if i == len(token) || token[i] != '0' && token[i] != '1' {
					return nil, errors.New("字段转义无效")
				}
			}
		}
		key := strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch item := value.(type) {
		case map[string]any:
			var exists bool
			value, exists = item[key]
			if !exists {
				return nil, errors.New("字段不存在")
			}
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(item) || strconv.Itoa(index) != key {
				return nil, errors.New("数组索引无效")
			}
			value = item[index]
		default:
			return nil, errors.New("字段不可继续访问")
		}
	}
	return value, nil
}

func fieldType(kind string, value any) bool {
	switch kind {
	case "null":
		return value == nil
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "number", "integer":
		var number float64
		switch n := value.(type) {
		case json.Number:
			if err := transportNumbers(n); err != nil {
				return false
			}
			exact, ok := new(big.Rat).SetString(string(n))
			if !ok || kind == "integer" && !exact.IsInt() {
				return false
			}
			parsed, err := n.Float64()
			if err != nil {
				return false
			}
			number = parsed
		case float64:
			number = n
		default:
			return false
		}
		return !math.IsNaN(number) && !math.IsInf(number, 0) && (kind == "number" || math.Trunc(number) == number)
	}
	return false
}

func transportNumbers(value any) error {
	switch item := value.(type) {
	case map[string]any:
		for key, v := range item {
			if err := transportNumbers(v); err != nil {
				return fmt.Errorf("字段 %s: %w", key, err)
			}
		}
	case []any:
		for i, v := range item {
			if err := transportNumbers(v); err != nil {
				return fmt.Errorf("数组项 %d: %w", i, err)
			}
		}
	case json.Number:
		f, err := item.Float64()
		if err != nil || math.Abs(f) > 9007199254740991 {
			return errors.New("数值超出精确传输范围，请用字符串表达大数")
		}
		before, ok := new(big.Rat).SetString(string(item))
		after, roundOK := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
		if !ok || !roundOK || before.Cmp(after) != 0 {
			return errors.New("数值精度无法保留，请用字符串表达高精度值")
		}
	}
	return nil
}
