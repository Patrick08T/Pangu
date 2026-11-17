package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func newOperationFileset(ctx context.Context, record *Run, base, repository string) (*Fileset, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	local, err := readPerforce(ctx, conn, record.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return &Fileset{Base: base, Head: record.HeadCommit, repository: repository, Changes: []Change{},
		localPerforce: local != nil && local.LocalRoot != ""}, nil
}

func (set *Fileset) git(ctx context.Context, args ...string) (string, error) {
	data, err := set.bytes(ctx, args...)
	return strings.TrimSpace(string(data)), err
}

func (set *Fileset) bytes(ctx context.Context, args ...string) ([]byte, error) {
	data, truncated, err := set.output(ctx, 16*1024*1024, args...)
	if err == nil && truncated {
		err = errors.New("Git 查询结果超过16MiB")
	}
	return data, err
}

func (set *Fileset) output(ctx context.Context, limit int, args ...string) ([]byte, bool, error) {
	if !set.localPerforce {
		return gitOutput(ctx, set.repository, nil, limit, args...)
	}
	return runLocalGitOutput(ctx, set.repository, limit, args...)
}
