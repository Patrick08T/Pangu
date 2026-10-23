package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func Reconcile(ctx context.Context, namespace string) (int, error) {
	root, err := spoolRoot(namespace)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return 0, err
	}
	defer dir.Close()
	count := 0
	var failures error
	for {
		if err := ctx.Err(); err != nil {
			return count, errors.Join(failures, err)
		}
		names, err := dir.ReadDir(200)
		for _, file := range names {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			n, replayErr := replayFile(ctx, root, namespace, file.Name())
			count += n
			failures = errors.Join(failures, replayErr)
		}
		if errors.Is(err, io.EOF) {
			return count, failures
		}
		if err != nil {
			return count, errors.Join(failures, err)
		}
	}
}

func replayFile(ctx context.Context, root *os.Root, namespace, name string) (int, error) {
	data, err := readPending(root, name)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var item pending
	if len(data) > 16384 || json.Unmarshal(data, &item) != nil || item.Namespace != namespace {
		return 0, errors.New("审计补录文件无效")
	}
	if _, err := uuid.Parse(item.Entry.ID); err != nil || !strings.HasPrefix(name, item.Entry.ID+".") {
		return 0, errors.New("审计补录标识不匹配")
	}
	item.Entry.Namespace = namespace
	if err := restore(ctx, item.Entry); err != nil {
		return 0, err
	}
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	return 1, nil
}

func restore(ctx context.Context, e Entry) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `INSERT INTO pangu.audit_events(id,namespace,actor_id,token_id,request_id,action,object_id,project_id,result_id,result_version,outcome,http_status,created_at,finished_at,target_id,role)
 VALUES($1,$2,$3,nullif($4,'')::uuid,$5,$6,nullif($7,'')::uuid,nullif($8,'')::uuid,nullif($9,'')::uuid,$10,$11,$12,$13,$14,nullif($15,'')::uuid,nullif($16,''))
 ON CONFLICT(id) DO UPDATE SET project_id=EXCLUDED.project_id,result_id=EXCLUDED.result_id,result_version=EXCLUDED.result_version,
 outcome=EXCLUDED.outcome,http_status=EXCLUDED.http_status,finished_at=EXCLUDED.finished_at,target_id=EXCLUDED.target_id,role=EXCLUDED.role
 WHERE audit_events.namespace=EXCLUDED.namespace AND audit_events.actor_id=EXCLUDED.actor_id AND audit_events.request_id=EXCLUDED.request_id
 AND audit_events.action=EXCLUDED.action AND audit_events.object_id IS NOT DISTINCT FROM EXCLUDED.object_id AND audit_events.token_id IS NOT DISTINCT FROM EXCLUDED.token_id
 AND audit_events.outcome='pending' AND EXCLUDED.outcome<>'pending'`, e.ID, e.Namespace, e.ActorID, e.TokenID, e.RequestID, e.Action, e.ObjectID, e.ProjectID, e.ResultID, e.ResultVersion, e.Outcome, e.HTTPStatus, e.CreatedAt, e.FinishedAt, e.TargetID, e.Role)
	if err != nil || result.RowsAffected() == 1 {
		return err
	}
	var same bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.audit_events WHERE id=$1 AND namespace=$2 AND actor_id=$3 AND request_id=$4 AND action=$5
 AND object_id IS NOT DISTINCT FROM nullif($6,'')::uuid AND token_id IS NOT DISTINCT FROM nullif($7,'')::uuid
 AND ($8='pending' OR (outcome=$8 AND http_status IS NOT DISTINCT FROM $9 AND project_id IS NOT DISTINCT FROM nullif($10,'')::uuid
 AND result_id IS NOT DISTINCT FROM nullif($11,'')::uuid AND result_version IS NOT DISTINCT FROM $12
 AND target_id IS NOT DISTINCT FROM nullif($13,'')::uuid AND role IS NOT DISTINCT FROM nullif($14,''))))`, e.ID, e.Namespace, e.ActorID, e.RequestID, e.Action, e.ObjectID, e.TokenID, e.Outcome, e.HTTPStatus, e.ProjectID, e.ResultID, e.ResultVersion, e.TargetID, e.Role).Scan(&same)
	if err == nil && !same {
		return errors.New("补录与已有审计终态冲突，保留文件供人工核对")
	}
	return err
}

func readPending(root *os.Root, name string) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, 16385))
}

func Maintain(ctx context.Context, namespace string) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		check, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := Reconcile(check, namespace)
		cancel()
		if err != nil {
			slog.Warn("审计补录尚未完成，将重试")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
