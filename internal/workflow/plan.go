package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ExecutionPlan struct {
	Changes     []PlanChange     `json:"changes"`
	Assignments []TaskAssignment `json:"assignments"`
}

type PlanChange struct {
	NodeID       string    `json:"node_id"`
	Remove       bool      `json:"remove,omitempty"`
	AgentVersion string    `json:"agent_version,omitempty"`
	DependsOn    *[]string `json:"depends_on,omitempty"`
	Task         *string   `json:"task,omitempty"`
}

type TaskAssignment struct {
	TaskID string `json:"task_id"`
	NodeID string `json:"node_id"`
}

func ParseExecutionPlan(body string) (*ExecutionPlan, error) {
	var report struct {
		Plan  json.RawMessage `json:"execution_plan"`
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		return nil, err
	}
	var plan ExecutionPlan
	decoder := json.NewDecoder(strings.NewReader(string(report.Plan)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("缺少或无效的 execution_plan: %w", err)
	}
	ids, assigned := map[string]bool{}, map[string]bool{}
	for _, task := range report.Tasks {
		if task.ID == "" || ids[task.ID] {
			return nil, fmt.Errorf("任务 ID 为空或重复")
		}
		ids[task.ID] = true
	}
	if len(ids) == 0 || len(plan.Assignments) > 256 || len(plan.Changes) > 32 {
		return nil, fmt.Errorf("执行计划缺少任务或超过规模限制")
	}
	for _, a := range plan.Assignments {
		if !ids[a.TaskID] || assigned[a.TaskID] || !identifier.MatchString(a.NodeID) {
			return nil, fmt.Errorf("任务 %s 的执行节点缺失、重复或无效", a.TaskID)
		}
		assigned[a.TaskID] = true
	}
	if len(ids) != len(assigned) {
		return nil, fmt.Errorf("每个任务必须明确分配给一个执行节点")
	}
	return &plan, nil
}

// BeforeGate 返回审批前必须完成的节点；调整不能改写这部分历史。
func BeforeGate(d Definition, gate string) (map[string]bool, error) {
	layers, err := d.Layers()
	if err != nil {
		return nil, err
	}
	before := map[string]bool{}
	for _, layer := range layers {
		for _, n := range layer {
			if n.ID == gate {
				return before, nil
			}
		}
		for _, n := range layer {
			before[n.ID] = true
		}
	}
	return nil, fmt.Errorf("审批节点不存在")
}
