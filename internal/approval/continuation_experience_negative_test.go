//go:build integration

package approval

import (
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestContinuationRejectsChangedFrozenPayload(t *testing.T) {
	cases := []struct {
		name   string
		change func(*workflow.NodeInput)
	}{
		{"removed", func(in *workflow.NodeInput) { in.Experiences = nil }},
		{"version", func(in *workflow.NodeInput) { in.Experiences[0].Reference.VersionID = uuid.NewString() }},
		{"body_hash", func(in *workflow.NodeInput) { in.Experiences[0].Reference.BodyHash = strings.Repeat("b", 64) }},
		{"delivery_revision", func(in *workflow.NodeInput) { in.Experiences[0].Reference.DeliveryRevision++ }},
		{"content", func(in *workflow.NodeInput) {
			in.Experiences[0].Content = []byte(`{"kind":"rule","practice":"unapproved"}`)
		}},
		{"conditions", func(in *workflow.NodeInput) { in.Experiences[0].Conditions = "unconditional" }},
		{"source", func(in *workflow.NodeInput) { in.Experiences[0].Sources[0].EvidenceHash = strings.Repeat("b", 64) }},
		{"request", func(in *workflow.NodeInput) { in.RequestID = uuid.NewString() }},
		{"parent_lineage", func(in *workflow.NodeInput) {
			out := in.Parents["plan"]
			out.Lineage = &workflow.OutputLineage{Version: 1, ArtifactID: uuid.NewString(), OutputHash: strings.Repeat("b", 64)}
			in.Parents["plan"] = out
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newContinuationMemoryFixture(t)
			parent, child, input := f.continuePlan(t)
			test.change(&input)
			if _, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input); err == nil {
				t.Fatal("changed input inherited approval")
			}
			f.noInheritedGate(t, parent, child.RunID)
		})
	}
}

func (f continuationMemoryFixture) noInheritedGate(t *testing.T, parent *Gate, run string) {
	t.Helper()
	old, err := Get(f.ctx, f.actor.Namespace, parent.ID)
	if err != nil || old.Status != "consumed" {
		t.Fatal("immutable parent history changed", old, err)
	}
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	var count int
	if err = conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.approvals WHERE namespace=$1 AND run_id=$2`, f.actor.Namespace, run).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed proof created a gate", count, err)
	}
}

func TestContinuationRechecksSourceWithdrawalAndGeneration(t *testing.T) {
	for _, kind := range []string{"source_withdrawn", "parent_invalidated", "child_generation"} {
		t.Run(kind, func(t *testing.T) {
			f := newContinuationMemoryFixture(t)
			parent, child, input := f.continuePlan(t)
			var err error
			switch kind {
			case "source_withdrawn":
				err = learning.WithdrawSource(f.ctx, f.scope, f.source.ID)
			case "parent_invalidated":
				_, err = Invalidate(f.ctx, f.actor.Namespace, parent.RunID, parent.NodeID, 1, "source_changed")
			case "child_generation":
				input.ApprovalGeneration = 2
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input); err == nil {
				t.Fatal("invalid proof inherited approval")
			}
			f.noInheritedGate(t, parent, child.RunID)
		})
	}
}
