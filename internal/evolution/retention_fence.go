package evolution

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func retentionStopFenceTx(ctx context.Context, tx pgx.Tx, intent *intentRecord, reason string, waiting bool) error {
	_, err := tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_retention}',coalesce(request->'_retention','{}'::jsonb)||jsonb_build_object('reason',$2::text,'awaiting_stop',$3::bool)) WHERE id=$1 AND generation=$4`, intent.ID, reason, waiting, intent.Generation)
	return err
}

// retentionHoldRollbackTx 维护和普通rollout入口共享持久停止栅栏。
func retentionHoldRollbackTx(ctx context.Context, tx pgx.Tx, intent *intentRecord) (bool, error) {
	var waiting bool
	err := tx.QueryRow(ctx, `SELECT coalesce((request#>>'{_retention,awaiting_stop}')::boolean,false) FROM pangu.evolution_intents WHERE id=$1 AND generation=$2`, intent.ID, intent.Generation).Scan(&waiting)
	if err != nil || !waiting {
		return false, err
	}
	if intent.State != "reconciling" {
		err = setIntentStateTx(ctx, tx, intent, "reconciling")
	}
	return true, err
}
