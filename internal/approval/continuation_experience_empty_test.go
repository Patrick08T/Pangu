//go:build integration

package approval

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestContinuationRequiresExplicitFrozenUsageOrDoubleEmptyLegacy(t *testing.T) {
	cases := []struct {
		name                                    string
		empty, parentUsage, childUsage, allowed bool
	}{
		{"both_verified_empty_legacy", true, false, false, true},
		{"empty_usage_pair", true, true, true, true},
		{"empty_legacy_parent", true, false, true, true},
		{"nonempty_child_missing_usage", false, true, false, false},
		{"nonempty_parent_missing_usage", false, false, true, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newContinuationMemoryFixture(t, test.empty)
			parent, child, input := f.continuePrepared(t, test.parentUsage, test.childUsage)
			before := f.usageCount(t, child.ID)
			gate, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input)
			if test.allowed {
				if err != nil || gate.Status != "approved" || gate.InheritedFrom != parent.ID {
					t.Fatal("verified empty legacy did not inherit", gate, err)
				}
			} else {
				if err == nil {
					t.Fatal("missing nonempty proof inherited approval")
				}
				f.noInheritedGate(t, parent, child.RunID)
			}
			if after := f.usageCount(t, child.ID); before != after {
				t.Fatal("inheritance minted a new usage", before, after)
			}
		})
	}
}

func (f continuationMemoryFixture) usageCount(t *testing.T, request string) int {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	var count int
	if err = conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND usage->'input'->>'run_id'=$3`, f.actor.Namespace, f.scope.ProjectID, request).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestContinuationRejectsUnprovenNewEmptySelection(t *testing.T) {
	f := newContinuationMemoryFixture(t, true)
	parent, child, input := f.continuePrepared(t, false, false, func() { f.activate(t) })
	input.Experiences = []workflow.Experience{}
	if _, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input); err == nil {
		t.Fatal("nonempty child watermark became empty legacy proof")
	}
	f.noInheritedGate(t, parent, child.RunID)
}
