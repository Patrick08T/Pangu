package workspace

import (
	"context"
	"errors"
)

// Internal safety tracking is complete within its own budgets; it must not use
// the separately limited presentation/export file list.
func localChangedPaths(ctx context.Context, repository, before, after string) ([]string, error) {
	if !commitID.MatchString(before) || !commitID.MatchString(after) {
		return nil, errors.New("快照修订标识无效")
	}
	paths := make([]string, 0)
	args := []string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", before, after, "--"}
	err := streamLocalGit(ctx, repository, args, func(record []byte) error {
		name := string(record)
		if err := validateLocalName(name); err != nil {
			return err
		}
		paths = append(paths, name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}
