package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

// QueuePerforceDelivery 只冻结数据库中的交付检查点；磁盘与 P4 操作交给绑定 Worker。
func QueuePerforceDelivery(ctx context.Context, id string) (*P4Delivery, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", "workspace:"+id).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		out, err := readP4Delivery(ctx, conn, id)
		if out == nil && err == nil {
			return nil, ErrLeased
		}
		return out, err
	}
	record, err := readRun(ctx, conn, id)
	if err != nil {
		return nil, err
	}
	if record.ExpiredAt != nil {
		return nil, ErrExpired
	}
	if !record.Ready {
		return nil, errors.New("运行工作区尚未准备完成")
	}
	s, err := readPerforce(ctx, conn, id)
	if err != nil {
		return nil, errors.New("运行没有 Perforce 来源")
	}
	checkpoint, client := record.HeadCommit, s.Client+"-review"
	if s.LocalRoot != "" {
		// 本地现场由 Worker 补齐中断尝试后冻结；API 进程不操作磁盘。
		checkpoint, client = "", s.Client
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.perforce_deliveries(run_id,checkpoint,client,status) VALUES($1,$2,$3,'preparing') ON CONFLICT(run_id) DO UPDATE SET status=CASE WHEN pangu.perforce_deliveries.status IN ('shelved','empty') THEN pangu.perforce_deliveries.status ELSE 'preparing' END,error='',updated_at=now()`, id, checkpoint, client)
	if err != nil {
		return nil, err
	}
	out, err := readP4Delivery(ctx, conn, id)
	if out != nil {
		out.Dispatch = out.Status != "shelved" && out.Status != "empty"
	}
	return out, err
}

func FailPerforceDelivery(id string, cause error) {
	// 失败回执独立持久化，调用方取消后也允许最多 5 秒收尾。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := database.Open(ctx)
	if err != nil {
		return
	}
	defer conn.Close(context.Background())
	message, _ := diagnostic.Text(cause.Error(), 4096)
	_, _ = conn.Exec(ctx, "UPDATE pangu.perforce_deliveries SET status='failed',error=$2,updated_at=now() WHERE run_id=$1 AND status NOT IN ('shelved','empty')", id, message)
}
