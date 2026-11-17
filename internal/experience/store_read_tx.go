package experience

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// ReadTx 不开启或提交事务；调用方必须先持有身份锁。它不授予生产注入资格。
func (s Store) ReadTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Version, error) {
	scope.Action = "version_read"
	if err := s.authorizeTx(ctx, tx, scope); err != nil {
		return Version{}, err
	}
	version, err := s.readVersionTx(ctx, tx, scope, id)
	if err != nil {
		return Version{}, err
	}
	if err = s.Authorize(ctx, tx, scope); err != nil {
		return Version{}, err
	}
	return version, nil
}
