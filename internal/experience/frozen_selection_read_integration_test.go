//go:build integration

package experience

import (
	"errors"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func frozenReadFixture(t *testing.T) (storeFixture, SelectionInput, Usage) {
	t.Helper()
	f := newStoreFixture(t)
	f.policy(t)
	f.serving(t)
	input := SelectionInput{ProjectID: f.scope.ProjectID, RunID: uuid.NewString(), NodeID: "build", Round: 1, Generation: 1, WatermarkGeneration: 1, ApprovalRevision: 1, BoundInputHash: strings.Repeat("a", 64), RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	if _, err := f.store.Freeze(f.ctx, f.scope, input.RunID, 1); err != nil {
		t.Fatal(err)
	}
	usage, versions, err := f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(versions) != 1 {
		t.Fatal("fixture did not select memory", usage, err)
	}
	return f, input, usage
}

func readFrozen(t *testing.T, f storeFixture, input SelectionInput) (Watermark, Usage, []Version, error) {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	return f.store.ReadFrozenSelectionTx(f.ctx, tx, f.scope, input)
}

func TestFrozenSelectionReadNeverCreatesOrReranksMissingUsage(t *testing.T) {
	f, input, original := frozenReadFixture(t)
	input.Text = "different ranking text must not rerank persisted identity"
	mark, usage, versions, err := readFrozen(t, f, input)
	if err != nil || usage.InjectionHash != original.InjectionHash || len(versions) != 1 {
		t.Fatal("persisted usage not reused", usage, err)
	}
	input.NodeID = "not-executed"
	missing, _, _, err := readFrozen(t, f, input)
	if !errors.Is(err, ErrNotFound) || missing.Hash != mark.Hash || len(missing.Versions) != 1 {
		t.Fatal("missing proof became empty selection", missing, err)
	}
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	var count int
	if err = conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID).Scan(&count); err != nil || count != 1 {
		t.Fatal("reader persisted new usage", count, err)
	}
}

func TestFrozenSelectionReadKeepsIndependentGenerationIdentities(t *testing.T) {
	f, input, original := frozenReadFixture(t)
	input.Generation = 2
	usage, _, err := f.store.Select(f.ctx, f.scope, input)
	if err != nil {
		t.Fatal(err)
	}
	_, read, versions, err := readFrozen(t, f, input)
	if err != nil || read.Input.Generation != 2 || len(versions) != 1 || read.InjectionHash != original.InjectionHash || usage.InjectionHash != original.InjectionHash {
		t.Fatal("valid independently persisted generation rejected", read, err)
	}
	input.Generation = 3
	if _, _, _, err = readFrozen(t, f, input); !errors.Is(err, ErrNotFound) {
		t.Fatal("reader borrowed proof from another generation", err)
	}
}
