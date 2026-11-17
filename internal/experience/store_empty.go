package experience

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// FreezeEmptyTx 仅供可信服务器目的路由使用；调用方必须已持有 identity 事务锁。
// purpose 必须是服务器决定的 business/learning/evaluation，不能信任请求载荷。
// 不调用授权/来源回调，不读取版本或正文，也不向调用方返回既有非空水位。
func (s Store) FreezeEmptyTx(ctx context.Context, tx pgx.Tx, scope Scope, runID string, generation int64, purpose string) (Watermark, error) {
	if s.frozenEmptyOnly {
		return Watermark{}, ErrForbidden
	}
	if tx == nil || scope.Namespace == "" || runID == "" || generation < 1 || !slices.Contains([]string{"business", "learning", "evaluation"}, purpose) {
		return Watermark{}, ErrInvalid
	}
	if _, err := uuid.Parse(scope.ProjectID); err != nil {
		return Watermark{}, ErrInvalid
	}
	if err := Lock(ctx, tx, scope.Namespace, scope.ProjectID); err != nil {
		return Watermark{}, err
	}
	mark, err := watermarkTx(ctx, tx, scope, runID, generation)
	if err == nil {
		if len(mark.Versions) > 0 {
			return Watermark{}, ErrForbidden
		}
		return mark, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Watermark{}, err
	}
	revision, err := emptyPolicyTx(ctx, tx, scope, purpose)
	if err != nil {
		return Watermark{}, err
	}
	mark = Watermark{RunID: runID, Generation: generation, PolicyRevision: revision, Versions: []FrozenVersion{}}
	mark.Hash = watermarkHash(mark)
	data, err := json.Marshal(mark)
	if err != nil {
		return Watermark{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_watermarks(namespace,project_id,run_id,generation,snapshot) VALUES($1,$2,$3,$4,$5)`, scope.Namespace, scope.ProjectID, runID, generation, data)
	if err != nil {
		return Watermark{}, storageError(err)
	}
	scope.Action = "watermark_empty"
	return mark, auditTx(ctx, tx, scope, runID, map[string]any{"purpose": purpose, "generation": generation, "hash": mark.Hash})
}

func emptyPolicyTx(ctx context.Context, tx pgx.Tx, scope Scope, purpose string) (int64, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.projects WHERE namespace=$1 AND id=$2)`, scope.Namespace, scope.ProjectID).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrForbidden
	}
	var revision int64
	var enabled bool
	err = tx.QueryRow(ctx, `SELECT revision,COALESCE((policy->>'enabled')::boolean,false) OR COALESCE((policy->>'read_enabled')::boolean,false) OR COALESCE((policy->>'auto_publish')::boolean,false)
	 FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, scope.Namespace, scope.ProjectID).Scan(&revision, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if purpose == "business" && enabled {
		return 0, ErrForbidden
	}
	return revision, nil
}
