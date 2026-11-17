package learning

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// ListCheckers is a bounded list of currently retained, unexpired checker definitions.
func ListCheckers(ctx context.Context, scope experience.Scope, limit, offset int) ([]Checker, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return nil, experience.ErrInvalid
	}
	return transaction(ctx, scope, "checker_read", func(tx pgx.Tx, scope experience.Scope) ([]Checker, error) {
		rows, err := tx.Query(ctx, `SELECT c.id FROM pangu.learning_checkers c JOIN pangu.learning_checker_bodies b ON b.checker_id=c.id
		 WHERE c.namespace=$1 AND c.project_id=$2 AND c.expires_at>clock_timestamp() ORDER BY c.created_at DESC,c.id LIMIT $3 OFFSET $4`,
			scope.Namespace, scope.ProjectID, limit, offset)
		if err != nil {
			return nil, err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return nil, err
		}
		out := make([]Checker, 0, len(ids))
		for _, id := range ids {
			checker, err := checkerTx(ctx, tx, scope, id)
			if err != nil {
				return nil, err
			}
			out = append(out, checker)
		}
		return out, nil
	})
}
