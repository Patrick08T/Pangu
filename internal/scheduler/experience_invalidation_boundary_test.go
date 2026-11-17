package scheduler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestExperienceInvalidationClassifiesOnlyKnownDependencies(t *testing.T) {
	for _, err := range []error{nil, context.Canceled, context.DeadlineExceeded, errors.New("storage disconnected"), &pgconn.PgError{Code: "42501", Message: "permission denied for relation"}} {
		if invalidExperienceDependency(err) {
			t.Fatalf("operational failure classified as dependency loss: %v", err)
		}
	}
	for _, err := range []error{experience.ErrInvalidated, experience.ErrNotFound, experience.ErrForbidden, experience.ErrInvalid, identity.ErrForbidden} {
		if !invalidExperienceDependency(fmt.Errorf("dependency: %w", err)) {
			t.Fatalf("known dependency error not recognized: %v", err)
		}
	}
}

func TestExperienceInvalidationFrozenCheckpointRequiresProof(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	f.runtime.request.Snapshot.Fork = &workflow.Fork{Checkpoints: map[string]string{"plan": "checkpoint"}}
	if err := f.runtime.auditFrozenCheckpoints(context.Background()); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("unproven checkpoint accepted: %v", err)
	}
	f.runtime.request.Snapshot.Fork.Reused = map[string]workflow.NodeOutput{"plan": f.parent}
	if err := f.runtime.auditFrozenCheckpoints(context.Background()); err != nil {
		t.Fatal("healthy frozen checkpoint", err)
	}
	rebuildExec(t, `UPDATE pangu.experience_states SET validity='revoked' WHERE version_id=$1`, f.version)
	if err := f.runtime.auditFrozenCheckpoints(context.Background()); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("revoked checkpoint accepted: %v", err)
	}
}
