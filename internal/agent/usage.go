package agent

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/usage"
	"os/exec"
)

func runCodexUsage(ctx context.Context, cmd *exec.Cmd, scope, model string, events *codexEvents) error {
	id, err := usage.Reserve(ctx, "codex", model, nil)
	if err != nil {
		return err
	}
	err = runCodexProcess(ctx, cmd, scope)
	return errors.Join(err, usage.Finish(id, events.Usage, err == nil))
}
