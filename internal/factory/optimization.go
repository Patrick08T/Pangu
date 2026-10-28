package factory

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"reflect"
)

func checkOptimization(ctx context.Context, ns string, in Input, b *catalog.Bundle) error {
	if in.Mode != "optimize" {
		return nil
	}
	target, err := catalog.Get(ctx, ns, in.TargetID)
	if err != nil {
		return err
	}
	if target.Revision != in.TargetRevision {
		return catalog.ErrConflict
	}
	if target.Kind == "agent" {
		if len(b.Agents) != 1 || len(b.Steps) != 0 || b.Agents[0].ObjectID != target.ID {
			return invalid("优化须保留选定 Agent；新增职责请使用修改模式")
		}
		return nodeWithin(target.Spec.Agent, b.Agents[0].Node)
	}
	if target.Spec.Flow == nil || b.ObjectID != target.ID {
		return invalid("优化须更新选定工作流")
	}
	before, after := target.Spec.Flow.Budget, b.Budget
	if before != nil && (after == nil || after.ModelCalls > before.ModelCalls || after.SearchCalls > before.SearchCalls || after.CodexSessions > before.CodexSessions) {
		return invalid("优化不能扩大运行预算")
	}
	return flowWithin(ctx, ns, target.Spec.Flow, b)
}

func flowWithin(ctx context.Context, ns string, base *catalog.Flow, b *catalog.Bundle) error {
	steps := map[string]catalog.Binding{}
	baseline := map[string]*workflow.Node{}
	gated := false
	for _, s := range base.Nodes {
		steps[s.ID] = s
		v, err := catalog.ReadVersion(ctx, ns, s.AgentVersion)
		if err != nil {
			return err
		}
		baseline[s.ID] = v.Spec.Agent
		gated = gated || s.Approval != nil || v.Spec.Agent.Approval != nil
	}
	if gated && len(base.Nodes) != len(b.Steps) {
		return invalid("有审批的优化必须保留所有步骤和依赖；结构调整请使用修改模式")
	}
	agents := map[string]catalog.AgentChange{}
	for _, a := range b.Agents {
		agents[a.Key] = a
	}
	used := map[string]bool{}
	for _, step := range b.Steps {
		old, ok := steps[step.ID]
		if !ok || !reflect.DeepEqual(old.Approval, step.Approval) {
			return invalid("优化不能新增职责或改变审批；请改用修改模式")
		}
		if gated && (!reflect.DeepEqual(old.DependsOn, step.DependsOn) || !reflect.DeepEqual(old.When, step.When)) {
			return invalid("优化不能绕过审批依赖或条件；结构调整请使用修改模式")
		}
		a := agents[step.Agent]
		node := a.Node
		if a.VersionID != "" {
			v, err := catalog.ReadVersion(ctx, ns, a.VersionID)
			if err != nil {
				return err
			}
			node = v.Spec.Agent
		}
		if err := nodeWithin(baseline[step.ID], node); err != nil {
			return err
		}
		used[step.Agent] = true
	}
	if len(used) != len(agents) {
		return invalid("优化不能创建流程外的额外 Agent")
	}
	return nil
}

func nodeWithin(before, after *workflow.Node) error {
	if before == nil || after == nil {
		return invalid("优化缺少可比较的 Agent")
	}
	a, b := before.EffectivePolicy(), after.EffectivePolicy()
	if a.Retries < b.Retries || before.ReadOnly && !after.ReadOnly {
		return invalid("优化不能扩大文件权限或重试次数")
	}
	if before.Executor != after.Executor || before.Role != after.Role || before.Mode != after.Mode || before.ConnectionVersion != after.ConnectionVersion || before.Model != after.Model {
		return invalid("优化不能更换执行器、角色或模型连接；请使用修改模式")
	}
	if !reflect.DeepEqual(before.Handoff, after.Handoff) || !reflect.DeepEqual(before.Approval, after.Approval) || !reflect.DeepEqual(before.OutputSchema, after.OutputSchema) {
		return invalid("优化不能改变交接契约、输出契约或审批门槛")
	}
	return nil
}
