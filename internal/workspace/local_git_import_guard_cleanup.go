package workspace

import (
	"context"
	"errors"
	"strings"
	"time"
)

func cleanupLocalImportWithGit(ctx context.Context, repository, temporary, final, commit string, cause error, run func(context.Context, string, ...string) (string, error)) error {
	if errors.Is(cause, errLocalGitStopUnconfirmed) {
		return cause
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	_, err := run(ctx, repository, "update-ref", "-d", temporary)
	cause = recordLocalGitOperationError(ctx, errors.Join(cause, err))
	if errors.Is(cause, errLocalGitStopUnconfirmed) {
		return cause
	}
	if cause != nil && final != "" && commit != "" {
		if !commitID.MatchString(commit) || strings.Trim(commit, "0") == "" {
			return errors.Join(cause, errors.New("本地快照清理缺少有效的引用身份"))
		}
		_, err = run(ctx, repository, "update-ref", "-d", final, commit)
		cause = errors.Join(cause, err)
	}
	return cause
}
