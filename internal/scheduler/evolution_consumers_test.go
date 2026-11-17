//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/google/uuid"
)

func TestEvolutionConsumersRequiresEveryRootAndRoleStopReceipt(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	root := f.submit(t, f.target.VersionIDs[0], false, false)
	role := f.submit(t, f.target.VersionIDs[0], true, false)
	if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
		t.Fatal("missing stop proof permitted erasure", ok, err)
	}
	f.stop(t, root.RunID)
	if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
		t.Fatal("role reference omitted", ok, err)
	}
	f.stop(t, role.RunID)
	if ok, err := f.guard(t, "stopped"); !ok || err != nil {
		t.Fatal("authoritative stop rejected", ok, err)
	}
}

func TestEvolutionConsumersActiveAttemptOverridesStopReceipt(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	item := f.submit(t, f.target.VersionIDs[0], false, false)
	attempt := execution.Attempt{RunID: item.RunID, NodeRunID: uuid.NewString(), NodeID: "work"}
	if err := execution.Begin(context.Background(), &attempt); err != nil {
		t.Fatal(err)
	}
	f.stop(t, item.RunID)
	if ok, err := f.guard(t, "stopped"); ok || err != nil {
		t.Fatal("active attempt not known-running", ok, err)
	}
}

func TestEvolutionConsumersPendingAndErasedLineageAreUnknown(t *testing.T) {
	for _, kind := range []string{"pending", "erased"} {
		t.Run(kind, func(t *testing.T) {
			f := newEvolutionConsumersFixture(t)
			item := f.submit(t, f.target.VersionIDs[0], true, kind == "pending")
			if kind == "erased" {
				f.exec(t, `UPDATE pangu.submissions SET snapshot='{"workflow":{}}',purged_at=now() WHERE id=$1`, item.ID)
			}
			if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
				t.Fatal("unproven consumer became stopped", ok, err)
			}
		})
	}
}

func TestEvolutionConsumersFindsTransitivePrivateDescendants(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	child := f.privateVersion(t, f.request.BaselineVersion, f.target.VersionIDs[0])
	grandchild := f.privateVersion(t, child, "")
	item := f.submit(t, grandchild, true, false)
	if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
		t.Fatal("private ancestor omitted", ok, err)
	}
	f.stop(t, item.RunID)
	if ok, err := f.guard(t, "stopped"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := f.guard(t, "unreferenced"); ok || err != nil {
		t.Fatal("retained private descendant erased", ok, err)
	}
}

func TestEvolutionConsumersIgnoresOtherProjects(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	other := f.sameNamespaceProject(t)
	other.submit(t, f.target.VersionIDs[0], true, false)
	if ok, err := f.guard(t, "stopped"); !ok || err != nil {
		t.Fatal("foreign consumer blocked or cancelled", ok, err)
	}
}

func TestEvolutionConsumersGuardRejectsUnknownPhaseAndSQLFailure(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	if ok, err := f.guard(t, "unknown"); ok || err == nil {
		t.Fatal("unknown phase permitted", ok, err)
	}
}
