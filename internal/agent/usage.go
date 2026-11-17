package agent

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"os/exec"
)

func runCodexUsage(ctx context.Context, cmd *exec.Cmd, scope, model string, events *codexEvents) error {
	id, err := usage.Reserve(ctx, "codex", model, nil)
	if err != nil {
		return err
	}
	err = runCodexProcess(ctx, cmd, scope)
	if recordErr := usage.Finish(id, events.Usage, err == nil); recordErr != nil {
		return &workflow.Failure{Kind: "usage", Cause: errors.Join(err, recordErr)}
	}
	return err
}
