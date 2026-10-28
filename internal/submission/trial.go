package submission

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"time"
)

type Trial struct {
	RequestID string    `json:"request_id"`
	RunID     string    `json:"run_id,omitempty"`
	Revision  int       `json:"revision"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

func TrialKey(key string) string { return "trial:" + digest([]byte(key)) }

// 请求键自带时限，历史清理后也不能把旧重试当作新任务。
func CheckTrialKey(key string) error {
	stamp, random, found := strings.Cut(key, ".")
	millis, err := strconv.ParseInt(stamp, 10, 64)
	if !found || err != nil || len(stamp) != 13 {
		return errors.New("试运行请求键必须为 Unix 毫秒时间戳.UUID")
	}
	if _, err = uuid.Parse(random); err != nil {
		return errors.New("试运行请求键的随机标识无效")
	}
	age := time.Since(time.UnixMilli(millis))
	if age > 89*24*time.Hour {
		return ErrExpired
	}
	if age < -5*time.Minute {
		return errors.New("请求时间超前，请校准设备时钟后重试")
	}
	return nil
}

func FindTrial(ctx context.Context, namespace, project, id string, revision int, input, key string) (*Record, error) {
	if err := CheckTrialKey(key); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return findTrial(ctx, conn, namespace, project, id, revision, input, key)
}

func findTrial(ctx context.Context, conn writer, namespace, project, id string, revision int, input, key string) (*Record, error) {
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND operation='submit' AND key_hash=$3", namespace, project, digest([]byte(TrialKey(key)))))
	if err != nil {
		return nil, err
	}
	var object, hash string
	var source int
	err = conn.QueryRow(ctx, `SELECT t.object_id::text,t.revision,s.input_hash FROM pangu.agent_trials t JOIN pangu.submissions s ON s.id=t.request_id WHERE t.request_id=$1`, item.ID).Scan(&object, &source, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if object != id || source != revision || hash != digest([]byte(input)) || item.CreatedBy != identity.Current(ctx).ID {
		return nil, ErrConflict
	}
	if time.Since(item.CreatedAt) > 89*24*time.Hour {
		return nil, ErrExpired
	}
	return item, nil
}

// PrepareTrial 与草稿写入互斥，提交意图和来源关联在同一事务中保存。
func PrepareTrial(ctx context.Context, namespace, id string, revision int, d workflow.Definition, key string) (*Record, error) {
	if err := CheckTrialKey(key); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return nil, err
	}
	if actor := identity.Current(ctx); actor.ID != "" {
		if err = identity.Authorize(ctx, actor, d.ProjectID, "run", ""); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "trial:"+namespace+":"+d.ProjectID+":"+key)
	if err != nil {
		return nil, err
	}
	previous, err := findTrial(ctx, tx, namespace, d.ProjectID, id, revision, d.Input, key)
	if err == nil {
		return previous, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	var current bool
	err = tx.QueryRow(ctx, `SELECT revision=$3 AND NOT archived AND kind='agent' FROM pangu.catalog_objects WHERE namespace=$1 AND id=$2 FOR SHARE`, namespace, id, revision).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !current {
		return nil, ErrConflict
	}
	item, err := prepare(ctx, tx, namespace, "submit", TrialKey(key), Snapshot{Workflow: d})
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO pangu.agent_trials(request_id,object_id,revision) VALUES($1,$2,$3)", item.ID, id, revision)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func Trials(ctx context.Context, namespace, id, project string, offset int) ([]Trial, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT s.id::text,coalesce(s.run_id::text,''),t.revision,coalesce(s.created_by::text,''),s.created_at,CASE WHEN s.run_id IS NULL THEN 'pending' ELSE 'accepted' END
 FROM pangu.agent_trials t JOIN pangu.submissions s ON s.id=t.request_id WHERE s.namespace=$1 AND t.object_id=$2 AND s.project_id=$3 ORDER BY s.created_at DESC,s.id DESC LIMIT 100 OFFSET $4`, namespace, id, project, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Trial{}
	for rows.Next() {
		var item Trial
		if err = rows.Scan(&item.RequestID, &item.RunID, &item.Revision, &item.CreatedBy, &item.CreatedAt, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
