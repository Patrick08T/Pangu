package workspace

import (
	"context"
	"errors"
)

func cleanupLocalImport(repository, temporary, final, commit string, cause error) error {
	return cleanupLocalImportContext(context.Background(), repository, temporary, final, commit, cause)
}

func cleanupLocalImportContext(ctx context.Context, repository, temporary, final, commit string, cause error) (result error) {
	if errors.Is(cause, errLocalGitStopUnconfirmed) {
		return recordLocalGitOperationError(ctx, cause)
	}
	ctx, finish, err := beginLocalGitOperation(context.WithoutCancel(ctx))
	if err != nil {
		return errors.Join(cause, err)
	}
	defer func() { result = finish(recordLocalGitOperationError(ctx, result)) }()
	return cleanupLocalImportWithGit(ctx, repository, temporary, final, commit, cause, runLocalGit)
}
