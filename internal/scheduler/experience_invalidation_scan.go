package scheduler

import (
	"context"
	"errors"
	"sync"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
)

type experienceApprovalCursor struct {
	sync.Mutex
	after string
}

var experienceApprovalCursors sync.Map

// The cursor survives interrupted maintenance passes; completed passes wrap.
// Gate state and cancellation remain durable in approval/execution, not here.
func reconcileExperienceApprovals(ctx context.Context, namespace string) error {
	value, _ := experienceApprovalCursors.LoadOrStore(namespace, &experienceApprovalCursor{})
	cursor := value.(*experienceApprovalCursor)
	cursor.Lock()
	defer cursor.Unlock()
	var failures []error
	for {
		ids, err := experienceApprovalPage(ctx, namespace, cursor.after)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		if len(ids) == 0 {
			cursor.after = ""
			return errors.Join(failures...)
		}
		for _, id := range ids {
			if err = ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			err = validateExperienceApproval(ctx, namespace, id)
			cursor.after = id
			if err != nil && !errors.Is(err, experience.ErrInvalidated) {
				failures = append(failures, err)
			}
		}
	}
}

func experienceApprovalPage(ctx context.Context, namespace, after string) ([]string, error) {
	conn, err := database.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(ctx, `SELECT a.id::text FROM pangu.approvals a WHERE a.namespace=$1 AND a.id::text>$2
 AND ((a.invalidated_at IS NULL AND (a.status IN ('waiting','approved','consumed') OR
 (a.status='changes_requested' AND a.subject->'node_input'->>'review_purpose'='evidence'))) OR (a.invalidated_at IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM pangu.run_cancellations c WHERE c.run_id=a.run_id))) ORDER BY a.id::text LIMIT 100`, namespace, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
