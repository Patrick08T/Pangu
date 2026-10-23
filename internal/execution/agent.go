package execution

import (
	"context"
	"errors"
	"os"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type owner struct {
	PID          int
	Stamp, Scope string
}

func Recover(ctx context.Context) error {
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT DISTINCT owner_pid,owner_stamp,process_scope FROM pangu.node_attempts
		WHERE finished_at IS NULL AND owner_host=$1 AND owner_stamp<>''`, host)
	if err != nil {
		return err
	}
	owners, err := pgx.CollectRows(rows, pgx.RowToStructByPos[owner])
	if err != nil {
		return err
	}
	var failures []error
	for _, item := range owners {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		stopped, err := agent.ConfirmStopped(item.PID, item.Stamp, item.Scope)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if !stopped {
			continue
		}
		_, err = conn.Exec(ctx, `UPDATE pangu.node_attempts SET finished_at=now(),status='failed',category='worker_crash',
			message='原 Worker 已退出；已核对进程创建身份并确认受控进程组停止，本次尝试中断'
			WHERE finished_at IS NULL AND owner_host=$1 AND owner_pid=$2 AND owner_stamp=$3 AND process_scope=$4`, host, item.PID, item.Stamp, item.Scope)
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
