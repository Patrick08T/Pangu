package execution

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

var ErrPaused = errors.New("运行已暂停，等待节点执行许可")
var ErrDebugConflict = errors.New("调试状态已变化、仍有活动节点或运行已停止")

type DebugState struct {
	Paused        bool   `json:"paused"`
	StepRequested bool   `json:"step_requested"`
	StepNode      string `json:"step_node"`
	Revision      int    `json:"revision"`
	Active        int    `json:"active"`
}
type DebugCommand struct {
	Action   string `json:"action"`
	Revision int    `json:"revision"`
}

func GetDebug(ctx context.Context, run string) (*DebugState, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return readDebug(ctx, conn, run)
}

func readDebug(ctx context.Context, conn interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, run string) (*DebugState, error) {
	out := &DebugState{}
	err := conn.QueryRow(ctx, `SELECT coalesce(d.paused,false),coalesce(d.step_requested,false),coalesce(d.step_node,''),coalesce(d.revision,0),
 (SELECT count(*) FROM pangu.node_attempts WHERE run_id=$1 AND finished_at IS NULL)
 FROM (SELECT $1::uuid AS id) r LEFT JOIN pangu.run_debug d ON d.run_id=r.id`, run).Scan(&out.Paused, &out.StepRequested, &out.StepNode, &out.Revision, &out.Active)
	return out, err
}

func debugLock(ctx context.Context, tx pgx.Tx, run string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", run)
	if err != nil {
		return err
	}
	var stopped bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)
 OR EXISTS(SELECT 1 FROM pangu.run_history WHERE run_id=$1)
 OR EXISTS(SELECT 1 FROM pangu.run_debug WHERE run_id=$1 AND expired)
 OR EXISTS(SELECT 1 FROM pangu.workspace_runs WHERE run_id=$1 AND expired_at IS NOT NULL)
 OR EXISTS(SELECT 1 FROM pangu.retention_records WHERE run_id=$1 AND status='purged')`, run).Scan(&stopped)
	if err == nil && stopped {
		return ErrCancelled
	}
	return err
}

func DispatchReady(ctx context.Context, run, node string) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	if err = debugLock(ctx, tx, run); err != nil {
		return false, err
	}
	state, err := readDebug(ctx, tx, run)
	if err != nil {
		return false, err
	}
	if !state.Paused || state.StepNode == node {
		return true, tx.Commit(ctx)
	}
	var dispatched bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.node_attempts WHERE run_id=$1 AND node_id=$2)", run, node).Scan(&dispatched)
	if err != nil {
		return false, err
	}
	// 已派发节点在 durable 重放时仍须取回原来的子运行。
	if dispatched {
		return true, tx.Commit(ctx)
	}
	if !state.StepRequested || state.Active > 0 || state.StepNode != "" {
		return false, nil
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.run_debug SET step_requested=false,step_node=$2,revision=revision+1,updated_at=now() WHERE run_id=$1", run, node)
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func consumeStep(ctx context.Context, tx pgx.Tx, item *Attempt) error {
	state, err := readDebug(ctx, tx, item.RunID)
	if err != nil {
		return err
	}
	if !state.Paused {
		return nil
	}
	if state.Active > 0 || (state.StepNode != item.NodeID && (!state.StepRequested || state.StepNode != "")) {
		return ErrPaused
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.run_debug SET step_requested=false,step_node='',revision=revision+1,updated_at=now() WHERE run_id=$1", item.RunID)
	return err
}
