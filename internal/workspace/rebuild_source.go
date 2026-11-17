package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/google/uuid"
)

// RebuildSource 校验来源身份、保留期和 Git 祖先证据，只返回原始干净基线。
// 不创建运行、不 checkout、不改变来源或取消状态；返回前释放 lease，实际重建须重新校验。
func RebuildSource(ctx context.Context, sourceRunID, projectID, namespace string) (string, error) {
	var origin string
	err := withRebuildSource(ctx, sourceRunID, projectID, namespace, func(_ *Project, _ *Run, clean string) error {
		origin = clean
		return nil
	})
	return origin, err
}

// withRebuildSource 在完整校验及 use 执行期间持有来源 lease，防止校验后 checkout 前被清理。
func withRebuildSource(ctx context.Context, sourceRunID, projectID, namespace string, use func(*Project, *Run, string) error) error {
	if _, err := uuid.Parse(sourceRunID); err != nil {
		return err
	}
	project, err := ProjectByID(ctx, namespace, projectID)
	if err != nil {
		return err
	}
	if project.WorkerID != WorkerID() || project.VCS != "git" {
		return errors.New("干净重建仅支持当前 Worker 的 Git 项目，不接管本地 P4 工作区")
	}
	release, err := Lease(ctx, sourceRunID)
	if err != nil {
		return err
	}
	defer release()
	source, err := RerunSource(ctx, sourceRunID, projectID)
	if err != nil {
		return err
	}
	origin, err := rebuildOrigin(ctx, sourceRunID, projectID, namespace)
	if err != nil {
		return err
	}
	if !commitID.MatchString(origin) {
		return errors.New("来源缺少有效的原始干净基线")
	}
	base := filepath.Join(source.Directory, "base")
	if _, err = git(ctx, base, "merge-base", "--is-ancestor", origin, source.BaseCommit); err != nil {
		return errors.New("原始干净基线缺失或不在来源提交链上，禁止替换为当前分支")
	}
	return use(project, source, origin)
}
