package catalog

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func validateFields(d *workflow.Definition) error {
	for _, node := range d.Nodes {
		for name, field := range node.Bindings {
			if err := knownField(d, field); err != nil {
				return invalid(fmt.Sprintf("节点 %s 参数 %s：%v", node.ID, name, err))
			}
		}
		if node.When != nil {
			if err := knownField(d, node.When.Field); err != nil {
				return invalid(fmt.Sprintf("节点 %s 执行条件：%v", node.ID, err))
			}
		}
	}
	return nil
}

func knownField(d *workflow.Definition, field workflow.FieldBinding) error {
	var shape *workflow.Shape
	if field.From == "$input" {
		shape = d.InputSchema
	} else {
		for _, node := range d.Nodes {
			if node.ID == field.From {
				shape = node.OutputSchema
				break
			}
		}
	}
	if shape == nil {
		return nil
	}
	if field.Pointer != "" {
		if !strings.HasPrefix(field.Pointer, "/") {
			return fmt.Errorf("字段路径无效")
		}
		for _, token := range strings.Split(field.Pointer[1:], "/") {
			if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(token, "~1", ""), "~0", ""), "~") {
				return fmt.Errorf("字段转义无效")
			}
			key := strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
			switch shape.Type {
			case "object":
				child, exists := shape.Properties[key]
				if !exists {
					return fmt.Errorf("字段不存在，请重新选择")
				}
				shape = &child
			case "array":
				i, err := strconv.Atoi(key)
				if err != nil || i < 0 || strconv.Itoa(i) != key || shape.Items == nil {
					return fmt.Errorf("列表字段路径无效")
				}
				shape = shape.Items
			default:
				return fmt.Errorf("字段不可继续访问")
			}
		}
	}
	if shape.Type != field.Type && !(shape.Type == "integer" && field.Type == "number") {
		return fmt.Errorf("引用字段的类型与声明不一致")
	}
	return nil
}
