package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Project struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Source       string   `json:"source"`
	Branch       string   `json:"branch"`
	WorkerID     string   `json:"worker_id"`
	Verification []string `json:"verification"`
}

func Register(ctx context.Context, namespace string, project Project) (*Project, error) {
	if err := project.Validate(); err != nil {
		return nil, err
	}
	if namespace == "" {
		return nil, errors.New("项目命名空间不能为空")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	project.ID = uuid.NewString()
	if project.Verification == nil {
		project.Verification = []string{}
	}
	command, _ := json.Marshal(project.Verification)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,verification) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		project.ID, namespace, project.Name, filepath.Clean(project.Source), project.Branch, project.WorkerID, command)
	if err != nil {
		return nil, errors.New("登记项目失败")
	}
	return &project, nil
}

func (project Project) Validate() error {
	if strings.TrimSpace(project.Name) == "" || len(project.Name) > 128 || !filepath.IsAbs(project.Source) ||
		strings.HasPrefix(project.Source, `\\`) || project.Branch == "" || project.WorkerID == "" || project.ID != "" {
		return errors.New("项目名称、本地绝对路径、分支和 Worker 标识不能为空，ID 由服务端分配")
	}
	if len(project.Verification) > 0 {
		if len(project.Verification) > 32 || !filepath.IsAbs(project.Verification[0]) {
			return errors.New("验证命令需要绝对程序路径，最多32个参数")
		}
		for _, arg := range project.Verification {
			if len(arg) > 1024 || strings.ContainsRune(arg, 0) || diagnostic.Sensitive([]byte(arg)) {
				return errors.New("验证参数超限或含凭据，不允许登记")
			}
		}
	}
	return nil
}

func Projects(ctx context.Context, namespace string) ([]Project, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT id::text,name,source,branch,worker_id,verification FROM pangu.projects WHERE namespace=$1 ORDER BY created_at,id", namespace)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Project])
	if items == nil && err == nil {
		items = []Project{}
	}
	return items, err
}

func ProjectByID(ctx context.Context, namespace, id string) (*Project, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	project := &Project{}
	err = conn.QueryRow(ctx, `SELECT id::text,name,source,branch,worker_id,verification FROM pangu.projects WHERE id=$1 AND namespace=$2`, id, namespace).
		Scan(&project.ID, &project.Name, &project.Source, &project.Branch, &project.WorkerID, &project.Verification)
	if err != nil {
		return nil, errors.New("项目不存在或不属于当前命名空间")
	}
	return project, nil
}

func WorkerID() string {
	if value := os.Getenv("PANGU_WORKER_ID"); value != "" {
		return value
	}
	name, _ := os.Hostname()
	return name
}

func workspaceRoot() (string, error) {
	root := os.Getenv("PANGU_WORKSPACE_ROOT")
	if !filepath.IsAbs(root) || strings.HasPrefix(root, `\\`) {
		return "", errors.New("PANGU_WORKSPACE_ROOT 必须是本地绝对路径")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	return existingPath(root)
}
