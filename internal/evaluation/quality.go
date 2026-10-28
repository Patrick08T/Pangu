package evaluation

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
)

type Quality struct {
	Tasks           int `json:"tasks"`
	Runs            int `json:"runs"`
	FirstReviewed   int `json:"first_reviewed"`
	FirstAccepted   int `json:"first_accepted"`
	ReviewedTasks   int `json:"reviewed_tasks"`
	AcceptedTasks   int `json:"accepted_tasks"`
	NeedsChanges    int `json:"needs_changes"`
	RejectedTasks   int `json:"rejected_tasks"`
	UnreviewedTasks int `json:"unreviewed_tasks"`
}

func ProjectQuality(ctx context.Context, namespace, project string) (*Quality, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var out Quality
	err = conn.QueryRow(ctx, `WITH runs AS (
 SELECT s.id,s.task_id,s.created_at,f.decision,f.first_decision,f.updated_at,
 row_number() OVER(PARTITION BY s.task_id ORDER BY s.created_at,s.id) AS attempt
 FROM pangu.submissions s LEFT JOIN pangu.feedback f ON f.run_id=s.run_id AND f.source='feishu_session'
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.run_id IS NOT NULL AND s.created_at>=now()-interval '90 days'
 ), latest AS (
 SELECT DISTINCT ON(task_id) task_id,decision FROM runs WHERE decision IS NOT NULL ORDER BY task_id,created_at DESC,id DESC
 ) SELECT
 (SELECT count(DISTINCT task_id) FROM runs),(SELECT count(*) FROM runs),
 (SELECT count(*) FROM runs WHERE id=task_id AND first_decision IS NOT NULL),
 (SELECT count(*) FROM runs WHERE id=task_id AND first_decision='accepted'),
 (SELECT count(*) FROM latest),(SELECT count(*) FROM latest WHERE decision='accepted'),
 (SELECT count(*) FROM latest WHERE decision='needs_changes'),(SELECT count(*) FROM latest WHERE decision='rejected')`, namespace, project).Scan(&out.Tasks, &out.Runs, &out.FirstReviewed, &out.FirstAccepted, &out.ReviewedTasks, &out.AcceptedTasks, &out.NeedsChanges, &out.RejectedTasks)
	out.UnreviewedTasks = out.Tasks - out.ReviewedTasks
	return &out, err
}
