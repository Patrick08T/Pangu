package scheduler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type p4Input struct {
	Project *workspace.Project `json:"project,omitempty"`
	RunID   string             `json:"run_id,omitempty"`
}
type p4Output struct {
	Check    *workflow.Preflight   `json:"check,omitempty"`
	Delivery *workspace.P4Delivery `json:"delivery,omitempty"`
}

func p4TaskName(worker string) string {
	return fmt.Sprintf("pangu-perforce-%x", sha256.Sum256([]byte(worker)))[:39]
}

func newPerforceTask(client *hatchet.Client) *hatchet.StandaloneTask {
	b := Backend{client: client}
	return client.NewStandaloneTask(p4TaskName(workspace.WorkerID()), func(ctx hatchet.Context, input p4Input) (out p4Output, err error) {
		defer func() {
			if err != nil && input.RunID != "" {
				workspace.FailPerforceDelivery(input.RunID, err)
			}
		}()
		if input.Project != nil {
			check, cancel := context.WithTimeout(ctx.GetContext(), 20*time.Second)
			defer cancel()
			project := input.Project
			if err := project.Validate(); err != nil {
				return p4Output{}, err
			}
			if project.WorkerID != workspace.WorkerID() || project.VCS != "perforce" {
				return p4Output{}, errors.New("来源类型或绑定 Worker 不匹配")
			}
			if _, err := perforce.CaptureLocal(check, *project.Perforce, project.Source); err != nil {
				return p4Output{}, err
			}
			return p4Output{Check: &workflow.Preflight{Ready: true, Checks: []workflow.Check{{Component: "perforce", Status: "ready"}}}}, nil
		}
		run, err := b.GetRun(ctx.GetContext(), input.RunID)
		if err != nil {
			return p4Output{}, err
		}
		if !run.StopConfirmed || run.LogsExpired {
			return p4Output{}, errors.New("仅能交付已经停止且未过期的运行")
		}
		result, err := workspace.DeliverPerforce(ctx.GetContext(), input.RunID)
		return p4Output{Delivery: result}, err
	}, hatchet.WithRetries(0), hatchet.WithExecutionTimeout(2*time.Hour), hatchet.WithScheduleTimeout(15*time.Second))
}

func (b Backend) CheckProject(ctx context.Context, project workspace.Project) (*workflow.Preflight, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if err := project.Validate(); err != nil {
		return nil, err
	}
	if project.VCS != "perforce" {
		return nil, errors.New("此检查只适用于 Perforce 来源")
	}
	out, err := b.p4Operation(ctx, project.WorkerID, p4Input{Project: &project})
	return out.Check, err
}

func (b Backend) DeliverPerforce(ctx context.Context, id string) (*workspace.P4Delivery, error) {
	run, err := b.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if !run.StopConfirmed || run.LogsExpired {
		return nil, errors.New("运行尚未停止或产物已过期")
	}
	project, err := workspace.ProjectByID(ctx, b.Namespace(), run.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.VCS != "perforce" {
		return nil, errors.New("当前运行不是 Perforce 项目")
	}
	out, err := workspace.QueuePerforceDelivery(ctx, id)
	if err != nil || out == nil || !out.Dispatch {
		return out, err
	}
	_, err = b.client.RunNoWait(ctx, p4TaskName(project.WorkerID), p4Input{RunID: id})
	if err != nil {
		workspace.FailPerforceDelivery(id, err)
	}
	return out, err
}

func (b Backend) p4Operation(ctx context.Context, worker string, input p4Input) (p4Output, error) {
	name := p4TaskName(worker)
	result, err := b.client.Run(ctx, name, input)
	if err != nil {
		return p4Output{}, errors.Join(errors.New("绑定 Worker 的 Perforce 操作未完成"), err)
	}
	var out p4Output
	err = result.TaskOutput(name).Into(&out)
	return out, err
}
