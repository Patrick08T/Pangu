package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
	"time"
)

const ModelsTaskName = "pangu-models-v1"

func newModelChoices(client *hatchet.Client) *hatchet.StandaloneTask {
	return client.NewStandaloneTask(ModelsTaskName, func(ctx hatchet.Context, _ struct{}) ([]workflow.ModelChoice, error) {
		return agent.CodexModels(ctx.GetContext())
	}, hatchet.WithRetries(0), hatchet.WithExecutionTimeout(8*time.Second), hatchet.WithScheduleTimeout(8*time.Second))
}

func (b Backend) CodexModels(ctx context.Context) ([]workflow.ModelChoice, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	online, err := b.onlineWorker(ctx)
	if err != nil {
		return nil, err
	}
	if !online {
		return nil, ErrNotReady
	}
	result, err := b.client.Run(ctx, ModelsTaskName, struct{}{})
	if err != nil {
		return nil, errors.Join(ErrNotReady, errors.New("无法读取 Worker 的 Codex 模型目录"))
	}
	var out []workflow.ModelChoice
	if err = result.TaskOutput(ModelsTaskName).Into(&out); err != nil {
		return nil, err
	}
	return out, nil
}
