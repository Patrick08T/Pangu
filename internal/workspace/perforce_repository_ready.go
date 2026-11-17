package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/jackc/pgx/v5"
)

func prepareLocalRun(ctx context.Context, conn *pgx.Conn, record *Run, s *perforce.Snapshot) (result error) {
	ctx, finish, err := beginLocalPerforceOperation(ctx, conn)
	if err != nil {
		return err
	}
	defer func() { result = finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, localSnapshotTimeout)
	defer cancel()
	base := filepath.Join(record.Directory, "base")
	if _, err := os.Lstat(base); os.IsNotExist(err) {
		// Complete a historical source+base preparation without mutating its source.
		_, err := publishLocalPreparation(ctx, s, record.Directory, record.BaseCommit, func(staging string) (string, error) {
			err := cloneLocalRepository(ctx, filepath.Join(record.Directory, "source"), record.BaseCommit, staging, localManifest{})
			return record.BaseCommit, err
		})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := inspectCompleteLocalRepository(ctx, base, record.BaseCommit); err != nil {
		return err
	}
	head, err := runLocalGit(ctx, base, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != record.BaseCommit {
		// A validated fork chooses its prior immutable checkpoint, not the new
		// equal-tree capture commit. Never reset an unrelated or successful head.
		equal, err := sameLocalTrees(ctx, base, head, record.BaseCommit)
		if err != nil || !equal || head != s.Commit {
			return errors.New("运行基准缺失或准备状态不一致")
		}
		if _, err := runLocalGit(ctx, base, "update-ref", "HEAD", record.BaseCommit, head); err != nil {
			return err
		}
	}
	if _, err := verifyLocalBinding(ctx, s); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.workspace_runs SET ready=true WHERE run_id=$1", record.ID)
	return err
}
