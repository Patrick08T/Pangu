package catalog

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func validateReferences(ctx context.Context, reader rowReader, namespace string, item NewObject, publishing bool) error {
	if publishing && item.Spec.Agent != nil {
		if err := item.Spec.Agent.Handoff.Validate(true); err != nil {
			return invalid(err.Error())
		}
	}
	if namespace == "" {
		return errors.New("命名空间不能为空")
	}
	if node := item.Spec.Agent; node != nil && item.Kind == "agent" {
		if publishing && node.Executor == "eino" && node.ConnectionVersion == "" {
			return invalid("发布 Eino Agent 必须指定共享模型连接版本")
		}
		if node.ConnectionVersion != "" {
			v, err := readVersion(ctx, reader, namespace, node.ConnectionVersion)
			if err != nil {
				return err
			}
			if v.Kind != "connection" || v.Archived || v.Spec.Connection.Model != node.Model {
				return invalid("模型连接类型错误、已禁用或模型不一致")
			}
		}
	}
	if item.Spec.Flow != nil {
		if !publishing {
			_, err := resolveFlow(ctx, reader, namespace, "", item.Spec.Flow, true)
			return err
		}
		_, err := compileFlow(ctx, reader, namespace, "", item.Spec.Flow, true)
		return err
	}
	return nil
}

func Compile(ctx context.Context, namespace, version, project, input string) (*workflow.Definition, error) {
	if _, err := uuid.Parse(project); err != nil {
		return nil, invalid("运行必须指定有效项目")
	}
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
		out = &workflow.Definition{ProjectID: project, Nodes: []workflow.Node{node}}
	case "workflow":
		out, err = compileFlow(ctx, conn, namespace, project, v.Spec.Flow, false)
	default:
		return nil, errors.New("仅可运行已发布的 Agent 或工作流版本")
	}
	if err != nil {
		return nil, err
	}
	out.Input, out.VersionID = input, v.ID
	if err := out.ValidateHandoffs(); err != nil {
		return nil, invalid(err.Error())
	}
	if err := out.ValidateInput(); err != nil {
		return nil, invalid(err.Error())
	}
	return out, nil
}

func compileFlow(ctx context.Context, reader rowReader, namespace, project string, flow *Flow, newReference bool) (*workflow.Definition, error) {
	out, err := resolveFlow(ctx, reader, namespace, project, flow, newReference)
	if err != nil {
		return nil, err
	}
	if err := out.ValidateHandoffs(); err != nil {
		return nil, invalid(err.Error())
	}
	if project == "" {
		_, err = out.TemplateLayers()
	} else {
		_, err = out.Layers()
	}
	if err != nil {
		return nil, invalid(err.Error())
	}
	if err = validateFields(out); err != nil {
		return nil, err
	}
	return out, nil
}

func resolveFlow(ctx context.Context, reader rowReader, namespace, project string, flow *Flow, newReference bool) (*workflow.Definition, error) {
	out := &workflow.Definition{ProjectID: project, InputSchema: flow.Parameters, Budget: flow.Budget}
	for _, binding := range flow.Nodes {
		v, err := readVersion(ctx, reader, namespace, binding.AgentVersion)
		if err != nil {
			return nil, err
		}
		if v.Kind != "agent" || v.Spec.Agent == nil || (newReference && v.Archived) {
			return nil, invalid("工作流必须引用当前工作空间内可用的 Agent 版本")
		}
		node := *v.Spec.Agent
		node.ID, node.DependsOn, node.AgentVersion = binding.ID, binding.DependsOn, v.ID
		node.Bindings = binding.Bindings
		node.When = binding.When
		node.ReadOnly = node.ReadOnly || binding.ReadOnly
		if binding.Task != "" {
			node.Instruction += "\n\n本步骤任务（在角色权限和交接契约内执行）：\n" + binding.Task
		}
		if binding.OutputSchema != nil {
			node.OutputSchema = binding.OutputSchema
		}
		if binding.Approval != nil {
			node.Approval = binding.Approval
		}
		out.Nodes = append(out.Nodes, node)
	}
	return out, nil
}
