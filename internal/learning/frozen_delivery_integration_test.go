//go:build integration

package learning

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func TestFreezeExcludesNonDeliverableMemoryBeforeReadingChangedFeedback(t *testing.T) {
	for _, mode := range []string{"shadow", "paused"} {
		t.Run(mode, func(t *testing.T) {
			f := newLearningFixture(t)
			version := f.frozenFeedback(t, mode)
			f.exec(t, `UPDATE pangu.feedback SET reason='corrected',revision=revision+1 WHERE id=$1`, version.Sources[0].ID)
			if _, err := f.store.Read(f.ctx, f.scope, version.ID); !errors.Is(err, experience.ErrInvalidated) {
				t.Fatal("fixture must have genuinely stale original feedback", err)
			}
			request := uuid.NewString()
			mark, err := f.store.Freeze(f.ctx, f.scope, request, 1)
			if err != nil || len(mark.Versions) != 0 || mark.Hash == "" {
				t.Fatal("never-injected memory blocked business acceptance", mark, err)
			}
			stored, err := f.store.ReadWatermark(f.ctx, f.scope, request, 1)
			if err != nil || stored.Hash != mark.Hash || len(stored.Versions) != 0 {
				t.Fatal("empty accepted watermark did not persist", stored, err)
			}
		})
	}
}

func TestFreezeStillValidatesDeliverableMemoryOriginalFeedback(t *testing.T) {
	for _, mode := range []string{"canary", "serving"} {
		t.Run(mode, func(t *testing.T) {
			f := newLearningFixture(t)
			version := f.frozenFeedback(t, mode)
			mark, err := f.store.Freeze(f.ctx, f.scope, uuid.NewString(), 1)
			if err != nil || len(mark.Versions) != 1 || mark.Versions[0].Reference.VersionID != version.ID {
				t.Fatal("eligible valid memory was dropped", mark, err)
			}
			f.exec(t, `UPDATE pangu.feedback SET reason='corrected',revision=revision+1 WHERE id=$1`, version.Sources[0].ID)
			if _, err := f.store.Freeze(f.ctx, f.scope, uuid.NewString(), 1); !errors.Is(err, experience.ErrInvalidated) {
				t.Fatal("eligible changed feedback no longer failed closed", err)
			}
		})
	}
}

func (f learningFixture) frozenFeedback(t *testing.T, mode string) experience.Version {
	t.Helper()
	policy := f.policy(t)
	source := f.feedback(t, f.report(t), "accepted")
	if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	content := claimContent()
	checker, err := RegisterChecker(f.ctx, f.scope, CheckerSpec{Kind: "json_assertion_v1", ContentHash: jsonHash(content),
		SourceKind: "feedback", JSONPointer: "/decision", Expected: []byte(`"accepted"`)}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	intent, err := f.store.EnsureLearningIntent(f.ctx, f.scope, experience.IntentInput{SourceRunID: source.RunID,
		EvidenceRevision: 2, PolicyRevision: policy.Revision, Purpose: "business", Terminal: true, StopConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	version, err := f.store.Create(f.ctx, f.scope, experience.Version{ProjectID: f.scope.ProjectID, Revision: 1,
		Content: content, Sources: []experience.Source{source}, ExpiresAt: time.Now().Add(30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := CheckClaim(f.ctx, f.scope, version.ID, checker.ID, intent.ID, intent.Generation)
	if err != nil {
		t.Fatal(err)
	}
	version, err = f.store.Activate(f.ctx, f.scope, version.ID, activation)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "shadow" {
		// Storage fixture for the freeze query, not evidence of a passed benefit experiment.
		data, _ := json.Marshal(experience.DeliveryPolicy{Mode: mode, TaskScope: []string{"business"}, TrafficBasisPoints: 10000})
		f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,2,$2,$3,$4)`, version.ID, mode, data, f.actor.ID)
	}
	return version
}
