package learning

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

// RetentionResult reports physical erasures separately from tombstoned identities.
type RetentionResult struct {
	Versions      int64 `json:"versions"`
	Bodies        int64 `json:"bodies"`
	CheckerBodies int64 `json:"checker_bodies"`
}

// PurgeExpiredTx is server-only maintenance, not a user-authorized endpoint. It
// takes identity then project locks, uses the database clock, and never commits.
// Each call processes at most limit version identities and limit checker bodies;
// callers may repeat bounded transactions. Disabled policy/actors do not extend TTL.
func PurgeExpiredTx(ctx context.Context, tx pgx.Tx, namespace, project string, limit int) (RetentionResult, error) {
	out := RetentionResult{}
	if tx == nil || namespace == "" || !validID(project) || limit < 1 || limit > 1000 {
		return out, experience.ErrInvalid
	}
	if err := identity.Lock(ctx, tx, namespace); err != nil {
		return out, err
	}
	if err := experience.Lock(ctx, tx, namespace, project); err != nil {
		return out, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.projects WHERE namespace=$1 AND id=$2)`, namespace, project).Scan(&exists); err != nil {
		return out, err
	}
	if !exists {
		return out, experience.ErrForbidden
	}
	targets, err := retentionTargetsTx(ctx, tx, namespace, project, limit)
	if err != nil {
		return out, err
	}
	for _, target := range targets {
		erased, err := experience.EraseVersionsTx(ctx, tx, namespace, project, []string{target.ID}, target.Reason)
		if err != nil {
			return RetentionResult{}, err
		}
		out.Versions += erased.Versions
		out.Bodies += erased.Bodies
	}
	out.CheckerBodies, err = purgeCheckerBodiesTx(ctx, tx, namespace, project, limit)
	if err != nil {
		return RetentionResult{}, err
	}
	return out, nil
}
