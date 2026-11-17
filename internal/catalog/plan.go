package catalog

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
	"reflect"
	"slices"
)

type Execution struct {
	BaseVersion string                    `json:"base_version"`
	ObjectID    string                    `json:"object_id"`
	Revision    int                       `json:"revision"`
	Flow        *Flow                     `json:"flow"`
	Workflow    workflow.Definition       `json:"workflow"`
	Changes     []GraphChange             `json:"changes"`
	Assignments []workflow.TaskAssignment `json:"assignments"`
}

type GraphChange struct {
	NodeID string   `json:"node_id"`
	Before *Binding `json:"before,omitempty"`
	After  *Binding `json:"after,omitempty"`
}

// PrepareExecution 从原始版本和当前草稿读取；审批主体冻结候选和具体差异。
func PrepareExecution(ctx context.Context, tx pgx.Tx, namespace string, d workflow.Definition, gate string, p workflow.ExecutionPlan) (*Execution, error) {
	base, err := readVersion(ctx, tx, namespace, d.VersionID)
	if err != nil {
		return nil, err
	}
	if base.Kind != "workflow" || base.Archived {
		return nil, ErrInvalid
	}
	before, err := workflow.BeforeGate(d, gate)
	if err != nil {
		return nil, err
	}
	flow, changes, err := patchPlan(base.Spec.Flow, p, before, gate)
	if err != nil {
		return nil, err
	}
	next, err := compileFlow(ctx, tx, namespace, d.ProjectID, flow, true)
	if err != nil {
		return nil, err
	}
	next.Input, next.VersionID = d.Input, d.VersionID
	if err = next.ValidateInput(); err != nil {
		return nil, err
	}
	if err = protectPlanPrefix(d, *next, before, gate); err != nil {
		return nil, err
	}
	item, err := scanObject(tx.QueryRow(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND id=$2", namespace, base.ObjectID))
	if err != nil {
		return nil, err
	}
	// 不覆盖与运行版本无关的草稿；先由配置编辑者解决分歧。
	if len(changes) > 0 && !reflect.DeepEqual(item.Spec, base.Spec) {
		return nil, invalid("当前工作流草稿与运行版本不同，请先发布一致版本后重新规划")
	}
	return &Execution{BaseVersion: base.ID, ObjectID: base.ObjectID, Revision: item.Revision, Flow: flow, Workflow: *next, Changes: changes, Assignments: p.Assignments}, nil
}

func patchPlan(original *Flow, plan workflow.ExecutionPlan, before map[string]bool, gate string) (*Flow, []GraphChange, error) {
	flow := *original
	flow.Nodes = slices.Clone(original.Nodes)
	changes, seen := []GraphChange{}, map[string]bool{}
	for _, c := range plan.Changes {
		if c.NodeID == "" || seen[c.NodeID] || before[c.NodeID] {
			return nil, nil, invalid("执行图不能重复修改节点或改写已完成规划")
		}
		seen[c.NodeID] = true
		index := slices.IndexFunc(flow.Nodes, func(n Binding) bool { return n.ID == c.NodeID })
		var old *Binding
		if index >= 0 {
			copy := flow.Nodes[index]
			old = &copy
		}
		if c.Remove {
			if old == nil || c.NodeID == gate || c.AgentVersion != "" || c.DependsOn != nil || c.Task != nil {
				return nil, nil, invalid("删除节点不允许同时修改，且不能删除审批节点")
			}
			flow.Nodes = slices.Delete(flow.Nodes, index, index+1)
			changes = append(changes, GraphChange{NodeID: c.NodeID, Before: old})
			continue
		}
		next := Binding{ID: c.NodeID}
		if old != nil {
			next = *old
		}
		if c.AgentVersion != "" {
			next.AgentVersion = c.AgentVersion
		}
		if c.DependsOn != nil {
			next.DependsOn = slices.Clone(*c.DependsOn)
		}
		if c.Task != nil {
			next.Task = *c.Task
		}
		if old == nil {
			flow.Nodes = append(flow.Nodes, next)
		} else {
			flow.Nodes[index] = next
		}
		if old == nil || !reflect.DeepEqual(*old, next) {
			changes = append(changes, GraphChange{NodeID: c.NodeID, Before: old, After: &next})
		}
	}
	assigned := map[string]bool{}
	for _, a := range plan.Assignments {
		if before[a.NodeID] || slices.IndexFunc(flow.Nodes, func(n Binding) bool { return n.ID == a.NodeID }) < 0 {
			return nil, nil, invalid("任务必须映射到后续真实执行节点")
		}
		assigned[a.NodeID] = true
	}
	for _, n := range flow.Nodes {
		if !before[n.ID] && !assigned[n.ID] {
			return nil, nil, invalid("执行节点 " + n.ID + " 缺少任务映射")
		}
	}
	if len(changes) > 0 {
		flow.Editor = nil
	}
	return &flow, changes, nil
}

func protectPlanPrefix(previous, next workflow.Definition, before map[string]bool, gate string) error {
	current, err := workflow.BeforeGate(next, gate)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before, current) {
		return invalid("新节点只能在审批后执行，不能插入已完成规划")
	}
	for _, old := range previous.Nodes {
		if !before[old.ID] {
			continue
		}
		index := slices.IndexFunc(next.Nodes, func(n workflow.Node) bool { return n.ID == old.ID })
		if index < 0 || !reflect.DeepEqual(old, next.Nodes[index]) {
			return invalid("执行计划改变了已完成节点")
		}
	}
	return nil
}

// PublishExecutionTx 与续跑提交、审批消费处于同一事务，防止半发布。
func PublishExecutionTx(ctx context.Context, tx pgx.Tx, namespace string, p *Execution) (*Version, error) {
	item, err := lockedObject(ctx, tx, namespace, p.ObjectID, p.Revision)
	if err != nil {
		return nil, err
	}
	updated, err := saveObject(ctx, tx, namespace, item.ID, item.Revision, item.Name, Spec{Flow: p.Flow})
	if err != nil {
		return nil, err
	}
	version, err := publishObject(ctx, tx, namespace, item.ID, updated.Revision)
	if err != nil {
		return nil, err
	}
	actual, err := compileFlow(ctx, tx, namespace, p.Workflow.ProjectID, version.Spec.Flow, true)
	if err != nil {
		return nil, err
	}
	actual.Input, actual.VersionID = p.Workflow.Input, p.Workflow.VersionID
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(p.Workflow)
	if string(a) != string(b) {
		return nil, invalid("发布内容与已批准执行图不同")
	}
	return version, nil
}
