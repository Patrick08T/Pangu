package catalog

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func validateReferences(ctx context.Context, reader rowReader, namespace string, item NewObject, publishing bool, previousTemplate string) error {
	if namespace == "" {
		return errors.New("命名空间不能为空")
	}
	if item.ProjectID != "" {
		var exists bool
		if err := reader.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.projects WHERE namespace=$1 AND id=$2)", namespace, item.ProjectID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	if item.Spec.TemplateVersion != "" {
		v, err := readVersion(ctx, reader, namespace, item.Spec.TemplateVersion)
		if err != nil {
			return err
		}
		if v.Kind != "template" || v.Archived && previousTemplate != v.ID {
			return invalid("新绑定来源必须为未归档的共享模板版本")
		}
	}
	if node := item.Spec.Agent; node != nil && item.Kind == "agent" {
		if publishing && node.Executor == "eino" && node.ConnectionVersion == "" {
			return invalid("发布 Eino Agent 必须指定项目模型连接版本")
		}
		if node.ConnectionVersion != "" {
			v, err := readVersion(ctx, reader, namespace, node.ConnectionVersion)
			if err != nil {
				return err
			}
			if v.Kind != "connection" || v.ProjectID != item.ProjectID || v.Archived || v.Spec.Connection.Model != node.Model {
				return invalid("模型连接不属于当前项目、已禁用或模型不一致")
			}
		}
	}
	if item.Spec.Flow != nil {
		_, err := compileFlow(ctx, reader, namespace, item.ProjectID, item.Spec.Flow, true)
		return err
	}
	return nil
}

func Bind(ctx context.Context, namespace, version, project, name, connection string) (*Object, error) {
	v, err := ReadVersion(ctx, namespace, version)
	if err != nil {
		return nil, err
	}
	if v.Kind != "template" || v.Archived {
		return nil, ErrArchived
	}
	node := *v.Spec.Agent
	node.ConnectionVersion = connection
	if connection != "" {
		c, err := ReadVersion(ctx, namespace, connection)
		if err != nil {
			return nil, err
		}
		if c.Kind != "connection" || c.ProjectID != project || c.Archived {
			return nil, errors.New("连接不属于目标项目或已禁用")
		}
		node.Model = c.Spec.Connection.Model
	}
	return Create(ctx, namespace, NewObject{Kind: "agent", ProjectID: project, Name: name, Spec: Spec{Agent: &node, TemplateVersion: version}})
}

func Compile(ctx context.Context, namespace, version, input string) (*workflow.Definition, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	v, err := readVersion(ctx, conn, namespace, version)
	if err != nil {
		return nil, err
	}
	if v.Archived {
		return nil, ErrArchived
	}
	var out *workflow.Definition
	switch v.Kind {
	case "agent":
		node := *v.Spec.Agent
		node.AgentVersion = v.ID
		out = &workflow.Definition{ProjectID: v.ProjectID, Nodes: []workflow.Node{node}}
	case "workflow":
		out, err = compileFlow(ctx, conn, namespace, v.ProjectID, v.Spec.Flow, false)
	default:
		return nil, errors.New("仅可运行已绑定项目的 Agent 或工作流版本")
	}
	if err != nil {
		return nil, err
	}
	out.Input, out.VersionID = input, v.ID
	return out, out.ValidateInput()
}

func compileFlow(ctx context.Context, reader rowReader, namespace, project string, flow *Flow, newReference bool) (*workflow.Definition, error) {
	out := &workflow.Definition{ProjectID: project, InputSchema: flow.Parameters}
	for _, binding := range flow.Nodes {
		v, err := readVersion(ctx, reader, namespace, binding.AgentVersion)
		if err != nil {
			return nil, err
		}
		if v.Kind != "agent" || v.ProjectID != project || (newReference && v.Archived) {
			return nil, errors.New("工作流节点必须引用同项目的可用 Agent 版本")
		}
		node := *v.Spec.Agent
		node.ID, node.DependsOn, node.AgentVersion = binding.ID, binding.DependsOn, v.ID
		node.Bindings = binding.Bindings
		out.Nodes = append(out.Nodes, node)
	}
	_, err := out.Layers()
	return out, err
}
