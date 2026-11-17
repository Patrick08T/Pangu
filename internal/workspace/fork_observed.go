package workspace

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// ImportObserver sees the final remapped output before publication, or the cached
// output on recovery. A cached output must be validated, never newly signed.
type ImportObserver func(context.Context, string, string, *workflow.NodeOutput, bool) error

func ImportCheckpointsObserved(ctx context.Context, run string, fork *workflow.Fork, order []string, observe ImportObserver) (map[string]workflow.NodeOutput, error) {
	if fork == nil || observe == nil || len(order) != len(fork.Reused) {
		return nil, errors.New("观察式导入需要完整节点顺序与验证者")
	}
	seen := map[string]bool{}
	for _, node := range order {
		if _, ok := fork.Reused[node]; !ok || seen[node] || fork.Checkpoints[node] == "" {
			return nil, errors.New("复用节点顺序或检查点无效")
		}
		seen[node] = true
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	out := map[string]workflow.NodeOutput{}
	for _, node := range order {
		value, err := importCheckpoint(ctx, conn, run, node, fork.Checkpoints[node], fork.Reused[node], fork.ReportNodes[node], fork.ReportHashes[node], observe)
		if err != nil {
			return nil, err
		}
		out[node] = value
	}
	return out, nil
}

func observeImported(ctx context.Context, node, commit string, value *workflow.NodeOutput, cached bool, observers []ImportObserver) error {
	if len(observers) == 0 {
		return nil
	}
	if len(observers) != 1 || observers[0] == nil {
		return errors.New("导入验证者无效")
	}
	return observers[0](ctx, node, commit, value, cached)
}
