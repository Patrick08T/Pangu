package workspace

import (
	"context"
	"errors"
	"path/filepath"
)

func initialCommit(ctx context.Context, project *Project, sourceRunID string) (string, error) {
	if sourceRunID == "" {
		return Resolve(ctx, project.Source, project.Branch)
	}
	source, err := RerunSource(ctx, sourceRunID, project.ID)
	if err != nil {
		return "", err
	}
	return source.BaseCommit, nil
}

func RerunSource(ctx context.Context, id, project string) (*Run, error) {
	record, err := Get(ctx, id)
	if err != nil {
		return nil, errors.New("原运行基准不可用，无法重跑")
	}
	if record.ExpiredAt != nil {
		return nil, ErrExpired
	}
	root, err := workspaceRoot()
	if err != nil || record.ProjectID != project || record.WorkerID != WorkerID() || record.Directory != filepath.Join(root, id) || !record.Ready {
		return nil, errors.New("原基准项目、Worker 或磁盘不匹配")
	}
	if _, err := git(ctx, filepath.Join(record.Directory, "base"), "cat-file", "-e", record.BaseCommit+"^{commit}"); err != nil {
		return nil, errors.New("原运行基准已缺失，不允许替换为最新代码")
	}
	return record, nil
}
