package experience

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"
)

// NewFrozenEmptyStore 仅供可信调度器读取已冻结空水位；不会给予匿名用户一般经验权限。
// authorize必须绑定原始请求、项目、NULL created_by和空水位，并拒绝普通会话/token身份。
func NewFrozenEmptyStore(authorize Authorizer) Store {
	return Store{Authorize: authorize, frozenEmptyOnly: true}
}

func (s Store) allowsScope(scope Scope) bool {
	if !s.frozenEmptyOnly {
		return scope.ActorID != ""
	}
	return scope.ActorID == "" && slices.Contains([]string{"select", "watermark_read", "artifact_read", "artifact_write", "reference_read"}, scope.Action)
}

func (s Store) readWatermarkTx(ctx context.Context, tx pgx.Tx, scope Scope, runID string, generation int64) (Watermark, error) {
	if !s.frozenEmptyOnly {
		return watermarkTx(ctx, tx, scope, runID, generation)
	}
	var mark Watermark
	err := scanJSON(tx.QueryRow(ctx, `SELECT snapshot FROM pangu.experience_watermarks
	 WHERE namespace=$1 AND project_id=$2 AND run_id=$3 AND generation=$4
	 AND snapshot->'versions'='[]'::jsonb`, scope.Namespace, scope.ProjectID, runID, generation), &mark)
	if err != nil {
		return Watermark{}, err
	}
	if len(mark.Versions) != 0 || mark.RunID != runID || mark.Generation != generation || watermarkHash(mark) != mark.Hash {
		return Watermark{}, ErrInvalidated
	}
	return mark, nil
}
