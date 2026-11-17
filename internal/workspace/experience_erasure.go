package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrExperienceErasureDeferred = errors.New("experience erasure lacks current authority, stopped receipt, or exclusive managed workspace")

// ExperienceErasureVerifier is a trusted server callback. It must prove the exact
// request/run's current immutable erasure cause in this transaction, without RPC.
// It is not a user authorization callback and must never return nil by default.
type ExperienceErasureVerifier func(context.Context, pgx.Tx) error

// EraseExperienceRun erases private copies after machine-confirmed termination.
// It preserves identities and hashes, and never treats cancellation as stop proof.
func EraseExperienceRun(ctx context.Context, namespace, project, request, run string, verify ExperienceErasureVerifier) error {
	if namespace == "" || verify == nil {
		return ErrExperienceErasureDeferred
	}
	for _, id := range []string{project, request, run} {
		if parsed, err := uuid.Parse(id); err != nil || parsed.String() != id {
			return ErrExperienceErasureDeferred
		}
	}
	s := experienceErasureScope{namespace, project, request, run}
	conn, err := experienceErasureLease(ctx, s)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	record, err := readRun(ctx, conn, run)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	root := ""
	if record != nil && err == nil {
		root, err = workspaceRoot()
		if err != nil {
			return err
		}
		if record.ProjectID != project || record.WorkerID != WorkerID() || !samePath(record.Directory, filepath.Join(root, run)) {
			return ErrExperienceErasureDeferred
		}
		if err = retainPerforce(ctx, conn, run, record.Directory); err != nil {
			return err
		}
	} else if err = experienceErasureAbsentWorkspace(ctx, conn, s); err != nil {
		return err
	}
	if err = experienceErasureIntent(ctx, conn, s, verify); err != nil {
		return err
	}
	if root != "" {
		if err = experienceErasureFiles(ctx, conn, root, run, record.Directory); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return experienceErasureClear(ctx, conn, s, verify)
}

func experienceErasureFiles(ctx context.Context, conn *pgx.Conn, root, run, directory string) error {
	if err := removePerforceClient(ctx, conn, run, directory); err != nil {
		return err
	}
	managed, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer managed.Close()
	if err = ctx.Err(); err != nil {
		return err
	}
	return managed.RemoveAll(run)
}
