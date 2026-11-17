package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type localPerforceOperationKey struct{}

func finishNoLocalOperation(err error) error { return err }

func beginLocalPerforceOperation(ctx context.Context, conn *pgx.Conn) (context.Context, func(error) error, error) {
	if err := checkLocalGitOperation(ctx); err != nil {
		return ctx, nil, err
	}
	if ctx.Value(localPerforceOperationKey{}) != true {
		if err := lockLocalWorkspace(ctx, conn); err != nil {
			return ctx, nil, err
		}
	}
	ctx, finish, err := beginLocalGitOperation(ctx)
	return context.WithValue(ctx, localPerforceOperationKey{}, true), finish, err
}

func beginProjectLocalOperation(ctx context.Context, conn *pgx.Conn, project *Project, id string) (context.Context, func(error) error, error) {
	if project.VCS != "perforce" {
		return ctx, finishNoLocalOperation, nil
	}
	return beginRunLocalOperation(ctx, conn, id, project.Source != "")
}

func beginRunLocalOperation(ctx context.Context, conn *pgx.Conn, id string, creating bool) (context.Context, func(error) error, error) {
	s, err := readPerforce(ctx, conn, id)
	if errors.Is(err, pgx.ErrNoRows) {
		if !creating {
			return ctx, finishNoLocalOperation, nil
		}
	} else if err != nil {
		return ctx, nil, err
	} else if s.LocalRoot == "" {
		return ctx, finishNoLocalOperation, nil
	}
	return beginLocalPerforceOperation(ctx, conn)
}

// Only local P4 uses the controlled Git runner; historical isolated P4 and Git
// retain their original command behavior and do not acquire a local guard.
func inspectRunCommit(ctx context.Context, conn *pgx.Conn, record *Run, commit string) (result error) {
	s, err := readPerforce(ctx, conn, record.ID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && s.LocalRoot == "" {
		_, err = git(ctx, filepath.Join(record.Directory, "base"), "cat-file", "-e", commit+"^{commit}")
		return err
	}
	if err != nil {
		return err
	}
	ctx, finish, err := beginLocalPerforceOperation(ctx, conn)
	if err != nil {
		return err
	}
	defer func() { result = finish(result) }()
	_, err = runLocalGit(ctx, filepath.Join(record.Directory, "base"), "cat-file", "-e", commit+"^{commit}")
	return err
}

func inspectRerunCommit(ctx context.Context, record *Run) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	return inspectRunCommit(ctx, conn, record, record.BaseCommit)
}
