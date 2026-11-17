package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

func experienceErasureAbsentWorkspace(ctx context.Context, conn *pgx.Conn, s experienceErasureScope) error {
	var worker string
	err := conn.QueryRow(ctx, `SELECT p.worker_id FROM pangu.projects p JOIN pangu.submissions r ON r.project_id=p.id AND r.namespace=p.namespace WHERE p.namespace=$1 AND p.id=$2 AND r.id=$3 AND r.run_id=$4`, s.namespace, s.project, s.request, s.run).Scan(&worker)
	if err == pgx.ErrNoRows {
		return ErrExperienceErasureDeferred
	}
	if err != nil {
		return err
	}
	if worker != WorkerID() {
		return ErrExperienceErasureDeferred
	}
	root, err := workspaceRoot()
	if err != nil {
		return err
	}
	// Workspace preparation may create source files before inserting its DB row.
	// Without that row we cannot attribute an existing directory safely.
	_, err = os.Lstat(filepath.Join(root, s.run))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrExperienceErasureDeferred
}
