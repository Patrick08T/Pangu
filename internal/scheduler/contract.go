package scheduler

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

type RunSummary struct {
	ProjectID  string     `json:"project_id,omitempty"`
	CreatedBy  string     `json:"created_by,omitempty"`
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type RunError struct {
	Category  string `json:"category"`
	Message   string `json:"message"`
	Truncated bool   `json:"truncated"`
}

type NodeRun struct {
	RunSummary
	NodeID   string    `json:"node_id"`
	TaskID   string    `json:"task_id"`
	Executor string    `json:"executor"`
	Attempt  int       `json:"attempt"`
	Output   string    `json:"output,omitempty"`
	Error    *RunError `json:"error,omitempty"`
}

type RunEvent struct {
	ID        int       `json:"id"`
	TaskID    string    `json:"task_id"`
	Attempt   int       `json:"attempt"`
	Type      string    `json:"type"`
	Time      time.Time `json:"time"`
	Message   string    `json:"message"`
	Truncated bool      `json:"truncated"`
}

type RunQuery struct {
	ProjectID string
	Offset    int64
	Limit     int64
	Status    string
	Since     time.Time
	Until     time.Time
}

type RunList struct {
	Items      []RunSummary `json:"items"`
	NextOffset *int64       `json:"next_offset,omitempty"`
	Since      time.Time    `json:"since"`
	Until      time.Time    `json:"until"`
}

func ParseRunQuery(values url.Values, now time.Time) (RunQuery, error) {
	q := RunQuery{Limit: 20, Since: now.Add(-90 * 24 * time.Hour), Until: now}
	for key, items := range values {
		if len(items) != 1 || items[0] == "" {
			return q, errors.New("筛选参数只能提供一个非空值")
		}
		value := items[0]
		var err error
		switch key {
		case "project_id":
			q.ProjectID = value
			_, err = uuid.Parse(value)
		case "offset":
			q.Offset, err = strconv.ParseInt(value, 10, 64)
		case "limit":
			q.Limit, err = strconv.ParseInt(value, 10, 64)
		case "status":
			q.Status = value
			if hatchetStatus(value) == "" && value != "cancelling" {
				err = errors.New("不支持的运行状态")
			}
		case "since":
			q.Since, err = time.Parse(time.RFC3339Nano, value)
		case "until":
			q.Until, err = time.Parse(time.RFC3339Nano, value)
		default:
			err = errors.New("未知筛选参数")
		}
		if err != nil {
			return q, errors.New("筛选参数无效: " + key)
		}
	}
	if q.Offset < 0 || q.Limit < 1 || q.Limit > 100 || !q.Since.Before(q.Until) || q.Until.After(now) {
		return q, errors.New("分页或时间范围无效：limit 为 1–100，offset 非负，since 早于 until，until 不超过当前时间")
	}
	return q, nil
}

func runStatus(status rest.V1TaskStatus) string {
	switch status {
	case "QUEUED":
		return "queued"
	case "RUNNING":
		return "running"
	case "COMPLETED":
		return "succeeded"
	case "FAILED":
		return "failed"
	case "CANCELLED":
		return "cancelled"
	default:
		return "unknown"
	}
}

func hatchetStatus(status string) rest.V1TaskStatus {
	for _, value := range []rest.V1TaskStatus{"QUEUED", "RUNNING", "COMPLETED", "FAILED", "CANCELLED"} {
		if runStatus(value) == status {
			return value
		}
	}
	return ""
}

func nodeRun(row rest.V1TaskSummary) NodeRun {
	out := NodeRun{RunSummary: taskSummary(row), TaskID: row.TaskExternalId.String(), Attempt: 1}
	if row.RetryCount != nil {
		out.Attempt = *row.RetryCount + 1
	}
	if row.AdditionalMetadata != nil {
		out.NodeID, _ = (*row.AdditionalMetadata)["pangu_node_id"].(string)
		out.Executor, _ = (*row.AdditionalMetadata)["pangu_executor"].(string)
	}
	output, _ := json.Marshal(row.Output)
	var result workflow.NodeOutput
	if json.Unmarshal(output, &result) == nil {
		out.Output, _ = diagnostic.Text(result.Text, 16*1024)
	}
	out.Error = runError(row.ErrorMessage)
	return out
}

func taskSummary(row rest.V1TaskSummary) RunSummary {
	out := RunSummary{ID: row.WorkflowRunExternalId.String(), Status: runStatus(row.Status),
		CreatedAt: row.CreatedAt, StartedAt: optionalTime(row.StartedAt), FinishedAt: optionalTime(row.FinishedAt)}
	if row.AdditionalMetadata != nil {
		out.ProjectID, _ = (*row.AdditionalMetadata)["pangu_project_id"].(string)
		out.CreatedBy, _ = (*row.AdditionalMetadata)["pangu_created_by"].(string)
	}
	return out
}

func optionalTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	return value
}

func runError(value *string) *RunError {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	message, truncated := diagnostic.Text(*value, 64*1024)
	return &RunError{Category: "execution", Message: message, Truncated: truncated}
}

func runEvent(row rest.V1TaskEvent) RunEvent {
	message, truncated := diagnostic.Text(row.Message, 64*1024)
	out := RunEvent{ID: row.Id, TaskID: row.TaskId.String(), Type: string(row.EventType),
		Time: row.Timestamp, Message: message, Truncated: truncated, Attempt: 1}
	if row.RetryCount != nil {
		out.Attempt = *row.RetryCount + 1
	}
	return out
}
