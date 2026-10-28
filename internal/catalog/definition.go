package catalog

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func CheckDefinition(ctx context.Context, namespace string, d workflow.Definition) error {
	if err := d.ValidateHandoffs(); err != nil {
		return invalid(err.Error())
	}
	if d.VersionID != "" {
		expected, err := Compile(ctx, namespace, d.VersionID, d.ProjectID, d.Input)
		if err != nil {
			return err
		}
		if !sameJSON(d, expected) {
			return invalid("提交内容与声明的发布版本不一致")
		}
		return nil
	}
	for _, node := range d.Nodes {
		if node.AgentVersion == "" {
			continue
		}
		v, err := ReadVersion(ctx, namespace, node.AgentVersion)
		if err != nil {
			return err
		}
		if v.Kind != "agent" || v.Archived {
			return invalid("节点引用的 Agent 版本不可用")
		}
		expected := *v.Spec.Agent
		expected.ID, expected.DependsOn, expected.Bindings, expected.AgentVersion = node.ID, node.DependsOn, node.Bindings, v.ID
		expected.When = node.When
		if expected.Approval == nil {
			expected.Approval = node.Approval
		}
		if !sameJSON(node, expected) {
			return invalid("节点内容与声明的 Agent 版本不一致")
		}
	}
	return nil
}

func sameJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && bytes.Equal(left, right)
}
