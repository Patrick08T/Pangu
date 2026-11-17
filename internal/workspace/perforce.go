package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/jackc/pgx/v5"
)

func readPerforce(ctx context.Context, conn *pgx.Conn, id string) (*perforce.Snapshot, error) {
	s := &perforce.Snapshot{}
	s.RunID = id
	err := conn.QueryRow(ctx, `SELECT config,template,baseline,client,commit_id,local_path,coalesce(work_change,0),local_commit FROM pangu.perforce_sources WHERE run_id=$1`, id).Scan(&s.Config, &s.Template, &s.Baseline, &s.Client, &s.Commit, &s.LocalRoot, &s.WorkChange, &s.LocalCommit)
	return s, err
}

func preparePerforce(ctx context.Context, conn *pgx.Conn, id, root string, project *Project, previous string) (string, error) {
	return prepareLocalPerforce(ctx, conn, id, root, project, previous)
}

// 已有隔离运行的来源快照仍供历史交付读取；新运行只使用本地绑定。
func prepareLocalPerforce(ctx context.Context, conn *pgx.Conn, id, root string, project *Project, previous string) (_ string, result error) {
	ctx, finish, err := beginLocalPerforceOperation(ctx, conn)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	if err := checkLocalProcesses(ctx, conn); err != nil {
		return "", err
	}
	if previous != "" {
		if _, err := RerunSource(ctx, previous, project.ID); err != nil {
			return "", err
		}
	}
	s, err := readPerforce(ctx, conn, id)
	if errors.Is(err, pgx.ErrNoRows) {
		s, err = perforce.CaptureLocal(ctx, *project.Perforce, project.Source)
		if err != nil {
			return "", err
		}
		s.RunID = id
		_, err = conn.Exec(ctx, `INSERT INTO pangu.perforce_sources(run_id,project_id,config,template,baseline,client,commit_id,local_path) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, project.ID, s.Config, s.Template, s.Baseline, s.Client, s.Commit, s.LocalRoot)
	}
	if err != nil {
		return "", err
	}
	if err = recoverLocalAttempts(ctx, conn, s); err != nil {
		return "", err
	}
	commit, err := prepareLocalObjectRepository(ctx, s, filepath.Join(root, id), previous, project.ID)
	if err == nil {
		_, err = conn.Exec(ctx, "UPDATE pangu.perforce_sources SET commit_id=$2,local_commit=$2 WHERE run_id=$1", id, commit)
	}
	return commit, err
}

func importLocalSource(ctx context.Context, s *perforce.Snapshot, source, previous, project string) (string, error) {
	if previous == "" {
		return importLocalPerforce(ctx, s, source)
	}
	prior, err := RerunSource(ctx, previous, project)
	if err != nil {
		return "", err
	}
	return importInitialLocalSnapshot(ctx, s, source, prior)
}

func inspectPerforce(root string) error {
	resolved, err := existingPath(root)
	if err != nil || !samePath(root, resolved) {
		return errors.New("Perforce 项目目录缺失或被替换为链接")
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.EqualFold(entry.Name(), ".git") || entry.Type()&os.ModeType != 0 && !entry.IsDir() {
			return errors.New("Perforce 范围含 Git 元数据、链接或特殊文件")
		}
		return nil
	})
}

func projectCommit(ctx context.Context, conn *pgx.Conn, id, root string, project *Project, previous string) (string, error) {
	if project.VCS == "perforce" {
		return preparePerforce(ctx, conn, id, root, project, previous)
	}
	return initialCommit(ctx, project, previous)
}

func projectSource(project *Project, record *Run) string {
	if project.VCS == "perforce" {
		return localObjectSource(record.Directory)
	}
	return project.Source
}

func cleanupUnpreparedPerforce(ctx context.Context, conn *pgx.Conn, id string) error {
	_, err := readPerforce(ctx, conn, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	root, err := workspaceRoot()
	if err != nil {
		return err
	}
	if err = removePerforceClient(ctx, conn, id, filepath.Join(root, id)); err != nil {
		return err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err = directory.RemoveAll(id); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, "DELETE FROM pangu.perforce_sources WHERE run_id=$1", id)
	return err
}
