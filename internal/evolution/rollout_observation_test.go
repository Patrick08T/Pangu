//go:build integration

package evolution

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rolloutEvidence struct{ samples map[string]BusinessEvidence }

func (e rolloutEvidence) ReadExecutionTx(context.Context, pgx.Tx, Scope, string) (MachineResult, error) {
	return MachineResult{}, ErrChecker
}
func (e rolloutEvidence) ReadObservationTx(_ context.Context, _ pgx.Tx, _ Scope, id string) (BusinessEvidence, error) {
	value, ok := e.samples[id]
	if !ok {
		return value, ErrChecker
	}
	return value, nil
}
func (f rolloutFixture) sample() BusinessEvidence {
	id := uuid.NewString()
	return BusinessEvidence{RunID: id, BaselineRunID: uuid.NewString(), EntryKey: "workbench:main", VersionID: f.candidate, BaselineVersionID: f.request.BaselineVersion, Purpose: "business", SourceHash: hashText(id), Terminal: true, StopConfirmed: true, CandidateScore: 1, BaselineScore: 1}
}
func (f rolloutFixture) observing(t *testing.T) {
	t.Helper()
	if _, err := f.service.Reconcile(context.Background(), f.scope, f.id, 1); err != nil {
		t.Fatal(err)
	}
}

func TestRolloutObservationsRequireRealComparableEvidence(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	ctx := context.Background()
	if _, err := f.service.Observe(ctx, f.scope, f.id, 1, uuid.NewString()); !errors.Is(err, ErrChecker) {
		t.Fatal("missing evidence accepted", err)
	}
	changes := []func(*BusinessEvidence){
		func(e *BusinessEvidence) { e.Purpose = "evaluation" }, func(e *BusinessEvidence) { e.Terminal = false },
		func(e *BusinessEvidence) { e.StopConfirmed = false }, func(e *BusinessEvidence) { e.BaselineRunID = "" },
		func(e *BusinessEvidence) { e.CandidateScore = math.NaN() }, func(e *BusinessEvidence) { e.BaselineVersionID = f.candidate },
		func(e *BusinessEvidence) { e.SourceHash = "" }, func(e *BusinessEvidence) { e.EntryKey = "workbench:other" },
	}
	for _, change := range changes {
		sample := f.sample()
		change(&sample)
		f.service.Evidence = rolloutEvidence{map[string]BusinessEvidence{sample.RunID: sample}}
		if _, err := f.service.Observe(ctx, f.scope, f.id, 1, sample.RunID); err == nil {
			t.Fatal("untrusted/noncomparable sample accepted", sample)
		}
	}
}

func TestRolloutStableRequiresWindowAndIndependentSamples(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	ctx := context.Background()
	one, two := f.sample(), f.sample()
	reader := rolloutEvidence{map[string]BusinessEvidence{one.RunID: one, two.RunID: two}}
	f.service.Evidence = reader
	if _, err := f.service.Observe(ctx, f.scope, f.id, 1, one.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Observe(ctx, f.scope, f.id, 1, one.RunID); err != nil {
		t.Fatal("idempotent sample retry", err)
	}
	rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '2 seconds' WHERE id=$1`, f.id)
	intent, err := f.service.Advance(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "observing" {
		t.Fatal("duplicate counted as independent", intent, err)
	}
	if _, err = f.service.Observe(ctx, f.scope, f.id, 1, two.RunID); err != nil {
		t.Fatal(err)
	}
	intent, err = f.service.Advance(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "stable" {
		t.Fatal("qualified observation not stabilized", intent, err)
	}
	b, claims := f.binding(t)
	if b.StableVersion != f.candidate || b.AdoptedVersion != f.candidate || b.IntentID != "" || claims != 0 {
		t.Fatal("stable pointers/claims inconsistent", b, claims)
	}
	if _, err = f.service.RequestRollback(ctx, f.scope, f.id, 1, "old intent"); !errors.Is(err, ErrConflict) {
		t.Fatal("released intent still controls targets", err)
	}
}

func TestRolloutInsufficientDeadlineAndHardFailureRollback(t *testing.T) {
	for _, hard := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "hard"}[hard], func(t *testing.T) {
			f := newRolloutFixture(t)
			f.observing(t)
			ctx := context.Background()
			if hard {
				sample := f.sample()
				sample.HardFailure = true
				f.service.Evidence = rolloutEvidence{map[string]BusinessEvidence{sample.RunID: sample}}
				if _, err := f.service.Observe(ctx, f.scope, f.id, 1, sample.RunID); err != nil {
					t.Fatal(err)
				}
			} else {
				rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '20 seconds' WHERE id=$1`, f.id)
			}
			intent, err := f.service.Advance(ctx, f.scope, f.id, 1)
			if err != nil || intent.State != "rolling_back" {
				t.Fatal("bad observation allowed adoption", intent, err)
			}
			intent, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
			want := "insufficient"
			if hard {
				want = "rolled_back"
			}
			if err != nil || intent.State != want {
				t.Fatal("wrong rollback conclusion", intent, err)
			}
		})
	}
}
