package workspace

import (
	"context"
	"errors"
	"os"
)

func cleanupLocalOperationPreparation(ctx context.Context, root *os.Root, name string, cause error) error {
	cause = recordLocalGitOperationError(ctx, cause)
	if err := checkLocalGitOperation(context.WithoutCancel(ctx)); err != nil {
		return errors.Join(cause, err)
	}
	return cleanupLocalPreparation(root, name, cause)
}
