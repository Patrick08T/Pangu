package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The source read fails after a successful current-generation read.
// This verifies fail-closed behavior without requiring a live database.
type missingGenerationSource struct{}

type generationGuardRow struct {
	count int
}

func (missingGenerationSource) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	return generationGuardRow{count: len(args)}
}

func (row generationGuardRow) Scan(dest ...any) error {
	if row.count == 3 && len(dest) == 2 {
		*dest[0].(*int) = 1
		*dest[1].(*bool) = true
		return nil
	}
	return pgx.ErrNoRows
}

func TestApprovalInheritedGenerationFailsClosedWhenSourceDisappears(t *testing.T) {
	gate := &Gate{ID: "child", RunID: "run", NodeID: "build", ApprovalGeneration: 1, InheritedFrom: "missing-source"}
	err := checkGateGeneration(context.Background(), missingGenerationSource{}, "namespace", gate)
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("missing inherited source did not revoke authorization", err)
	}
}
