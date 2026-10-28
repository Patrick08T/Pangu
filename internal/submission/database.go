package submission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/identity"
	"regexp"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConflict = errors.New("同一幂等键已用于不同请求内容")
var ErrNotFound = errors.New("提交请求不存在或不属于当前命名空间")
var ErrExpired = errors.New("提交记录已超出安全重试窗口，禁止重新派发")
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type Snapshot struct {
	Fork        *workflow.Fork      `json:"fork,omitempty"`
	Workflow    workflow.Definition `json:"workflow"`
	CreatedBy   string              `json:"created_by,omitempty"`
	SourceRunID string              `json:"source_run_id,omitempty"`
}

type Record struct {
	TaskID    string    `json:"task_id"`
	CreatedBy string    `json:"created_by,omitempty"`
	ID        string    `json:"request_id"`
	ProjectID string    `json:"project_id"`
	Operation string    `json:"operation"`
	RunID     string    `json:"run_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Snapshot  Snapshot  `json:"-"`
}

const columns = `id::text,project_id::text,operation,coalesce(run_id::text,''),created_at,snapshot,payload_hash,coalesce(created_by::text,''),task_id::text`

func ValidKey(key string) bool { return keyPattern.MatchString(key) }

func PublicKey(key string) bool {
	return ValidKey(key) && !strings.HasPrefix(key, "delivery:") && !strings.HasPrefix(key, "eval:") && !strings.HasPrefix(key, "trial:") && !strings.HasPrefix(key, "factory:")
}

func digest(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func Prepare(ctx context.Context, namespace, operation, key string, snapshot Snapshot) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return prepare(ctx, conn, namespace, operation, key, snapshot)
}

type writer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func PrepareTx(ctx context.Context, tx pgx.Tx, namespace, operation, key string, snapshot Snapshot) (*Record, error) {
	return prepare(ctx, tx, namespace, operation, key, snapshot)
}

func prepare(ctx context.Context, conn writer, namespace, operation, key string, snapshot Snapshot) (*Record, error) {
	if !ValidKey(key) || namespace == "" || operation == "" {
		return nil, errors.New("提交作用域或幂等键无效")
	}
	if _, err := uuid.Parse(snapshot.Workflow.ProjectID); err != nil {
		return nil, errors.New("提交必须绑定项目")
	}
	if actor := identity.Current(ctx); actor.ID != "" {
		snapshot.CreatedBy = actor.ID
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,created_by,input_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,'')::uuid,$9) ON CONFLICT(namespace,project_id,operation,key_hash) DO NOTHING`,
		uuid.NewString(), namespace, snapshot.Workflow.ProjectID, operation, digest([]byte(key)), digest(data), data, snapshot.CreatedBy, digest([]byte(snapshot.Workflow.Input)))
	if err != nil {
		return nil, err
	}
	return find(ctx, conn, namespace, operation, key, snapshot.Workflow.ProjectID, digest(data))
}

func Find(ctx context.Context, namespace, operation, key string, snapshot Snapshot) (*Record, error) {
	if actor := identity.Current(ctx); actor.ID != "" {
		snapshot.CreatedBy = actor.ID
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return find(ctx, conn, namespace, operation, key, snapshot.Workflow.ProjectID, digest(data))
}

func find(ctx context.Context, conn writer, namespace, operation, key, project, hash string) (*Record, error) {
	item, storedHash, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND operation=$3 AND key_hash=$4", namespace, project, operation, digest([]byte(key))))
	if err == nil && storedHash != hash {
		return nil, ErrConflict
	}
	return item, err
}

func Get(ctx context.Context, namespace, id string) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND id=$2", namespace, id))
	return item, err
}

func ForRun(ctx context.Context, namespace, run string) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND run_id=$2", namespace, run))
	return item, err
}

func scan(row pgx.Row) (*Record, string, error) {
	item := &Record{Status: "pending"}
	var data []byte
	var hash string
	err := row.Scan(&item.ID, &item.ProjectID, &item.Operation, &item.RunID, &item.CreatedAt, &data, &hash, &item.CreatedBy, &item.TaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if err = json.Unmarshal(data, &item.Snapshot); err != nil {
		return nil, "", err
	}
	if item.RunID != "" {
		item.Status = "accepted"
	}
	return item, hash, nil
}
