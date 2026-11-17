//go:build integration

package learning

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestLearningIntentUsesStoredBusinessEvidence(t *testing.T) {
	f := newLearningFixture(t)
	policy := f.policy(t)
	source := f.report(t)
	input := experience.IntentInput{SourceRunID: source.RunID, EvidenceRevision: 1, PolicyRevision: policy.Revision,
		Purpose: "business", Terminal: true, StopConfirmed: true}
	if _, err := f.store.EnsureLearningIntent(f.ctx, f.scope, input); err != nil {
		t.Fatal(err)
	}
	input.EvidenceRevision++
	if _, err := f.store.EnsureLearningIntent(f.ctx, f.scope, input); !errors.Is(err, experience.ErrConflict) {
		t.Fatal("invented evidence revision", err)
	}
	input.EvidenceRevision = 1
	f.exec(t, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{purpose}','"learning"') WHERE run_id=$1`, source.RunID)
	if _, err := f.store.EnsureLearningIntent(f.ctx, f.scope, input); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("learning spawned learning", err)
	}
}
