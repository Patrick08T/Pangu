package evolution

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/experience"
	"testing"
	"time"
)

func TestObservationRequiresWindowSamplesAndBoundedDeadline(t *testing.T) {
	start := time.Unix(1000, 0)
	policy := experience.ObservationPolicy{WindowSeconds: 60, MinimumSamples: 2, DeadlineSeconds: 120, DegradationThreshold: 0.1}
	cases := []struct {
		name    string
		seconds int64
		samples []Observation
		want    string
	}{
		{"too_early", 30, []Observation{{Delta: 0}, {Delta: 0}}, "observing"},
		{"insufficient", 90, []Observation{{Delta: 0}}, "observing"},
		{"deadline", 120, []Observation{{Delta: 0}}, "insufficient"},
		{"stable", 90, []Observation{{Delta: 0}, {Delta: 0.2}}, "stable"},
		{"regression", 90, []Observation{{Delta: -0.3}, {Delta: 0}}, "rollback"},
		{"hard_gate", 1, []Observation{{HardFailure: true}}, "rollback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := observationOutcome(policy, start, start.Add(time.Duration(tc.seconds)*time.Second), tc.samples)
			if err != nil || got != tc.want {
				t.Fatalf("got=%s err=%v want=%s", got, err, tc.want)
			}
		})
	}
}

func TestAdoptionCASPreservesStableAndHumanEdits(t *testing.T) {
	old := Binding{EntryKey: "workbench", Revision: 7, AdoptedVersion: "v1", StableVersion: "v1"}
	next, err := adoptBinding(old, old, "intent", 1, "v2")
	if err != nil || next.AdoptedVersion != "v2" || next.StableVersion != "v1" || next.Revision != 8 {
		t.Fatalf("首次采用错误: %+v %v", next, err)
	}
	changed := old
	changed.Revision = 8
	changed.AdoptedVersion = "human-v3"
	if _, err = adoptBinding(changed, old, "intent", 1, "v2"); !errors.Is(err, ErrConflict) {
		t.Fatal("覆盖了人工变更")
	}
	stable, err := finishBinding(next, "intent", 1, true)
	if err != nil || stable.StableVersion != "v2" || stable.Revision != 9 {
		t.Fatalf("稳定晋级错误: %+v %v", stable, err)
	}
	rolled, err := finishBinding(next, "intent", 1, false)
	if err != nil || rolled.AdoptedVersion != "v1" || rolled.StableVersion != "v1" {
		t.Fatalf("回退错误: %+v %v", rolled, err)
	}
	if _, err = finishBinding(next, "intent", 2, false); !errors.Is(err, ErrConflict) {
		t.Fatal("接受旧代际")
	}
	if _, err = finishBinding(changed, "intent", 1, false); !errors.Is(err, ErrConflict) {
		t.Fatal("回退人工变更")
	}
}
