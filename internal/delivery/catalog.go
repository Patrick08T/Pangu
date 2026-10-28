package delivery

import (
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

var ErrInvalid = errors.New("交付草案参数无效：需要实施/只读审查版本、执行项目、设计报告及完整任务依赖")
var ErrConflict = errors.New("交付草案内容或请求键已变化")
var ErrNotFound = errors.New("交付草案不存在")
var ErrExpired = errors.New("交付草案或来源报告正文已过期")

type CreateInput struct {
	SubjectHash           string   `json:"subject_hash"`
	TaskIDs               []string `json:"task_ids"`
	ImplementationVersion string   `json:"implementation_version"`
	ReviewVersion         string   `json:"review_version"`
}

func Build(id, source, project string, input CreateInput, report research.Detail, implementation, review catalog.Version) (*workflow.Definition, error) {
	if report.Data == nil || report.Data.Mode != "design" || report.SubjectHash != input.SubjectHash || len(input.TaskIDs) < 1 || len(input.TaskIDs) > 32 {
		return nil, ErrInvalid
	}
	for _, version := range []catalog.Version{implementation, review} {
		if version.Kind != "agent" || version.Archived || version.Spec.Agent == nil || version.Spec.Agent.Executor != "codex" {
			return nil, ErrInvalid
		}
	}
	if implementation.Spec.Agent.ReadOnly || !review.Spec.Agent.ReadOnly {
		return nil, ErrInvalid
	}
	tasks, err := selectTasks(report.Data.Tasks, input.TaskIDs)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(struct {
		DeliveryID  string          `json:"delivery_id"`
		SourceRunID string          `json:"source_run_id"`
		ReportID    string          `json:"report_id"`
		ReportHash  string          `json:"report_hash"`
		Tasks       []research.Task `json:"tasks"`
	}{id, source, report.ID, report.SubjectHash, tasks})
	if err != nil {
		return nil, err
	}
	impl, check := *implementation.Spec.Agent, *review.Spec.Agent
	impl.ID, impl.AgentVersion = "implementation", implementation.ID
	if impl.Approval == nil {
		impl.Approval = &workflow.Approval{TimeoutSeconds: 3600}
	}
	check.ID, check.AgentVersion, check.DependsOn = "review", review.ID, []string{"implementation"}
	d := &workflow.Definition{ProjectID: project, Input: string(payload), Nodes: []workflow.Node{impl, check}}
	data, err := json.Marshal(d)
	if err != nil || len(data) > 256*1024 {
		return nil, ErrInvalid
	}
	if _, err = d.Layers(); err != nil {
		return nil, err
	}
	return d, d.ValidateInput()
}

func selectTasks(tasks []research.Task, ids []string) ([]research.Task, error) {
	selected := map[string]bool{}
	for _, id := range ids {
		if selected[id] {
			return nil, ErrInvalid
		}
		selected[id] = true
	}
	out := []research.Task{}
	for _, task := range tasks {
		if !selected[task.ID] {
			continue
		}
		for _, parent := range task.DependsOn {
			if !selected[parent] {
				return nil, ErrInvalid
			}
		}
		out = append(out, task)
	}
	if len(out) != len(selected) {
		return nil, ErrInvalid
	}
	return out, nil
}
