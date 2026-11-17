package scheduler

import (
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestExperienceRetentionSeconds(t *testing.T) {
	policy := func(seconds int64, authorized bool) []byte {
		raw, _ := json.Marshal(experience.Policy{Observation: experience.ObservationPolicy{RetentionSeconds: seconds, SourceRetentionAuthorized: authorized}})
		return raw
	}
	for _, tc := range []struct {
		name    string
		current []byte
		want    int64
	}{
		{"same", policy(3600, true), 3600},
		{"shorter", policy(100, true), 100},
		{"longer", policy(7200, true), 3600},
		{"disabled_without_new_grant", []byte(`{"enabled":false}`), 3600},
		{"no_invented_grant", policy(100, false), 3600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := experienceRetentionSeconds(policy(3600, true), tc.current)
			if err != nil || got != tc.want {
				t.Fatalf("seconds=%d err=%v want=%d", got, err, tc.want)
			}
		})
	}
	for _, raw := range [][]byte{policy(0, true), policy(3600, false), []byte(`{`)} {
		if _, err := experienceRetentionSeconds(raw, policy(3600, true)); err == nil {
			t.Fatal("unknown frozen authorization accepted")
		}
	}
	if _, err := experienceRetentionSeconds(policy(3600, true), []byte(`{"observation":{"retention_seconds":"unknown"}}`)); err == nil {
		t.Fatal("malformed current authority accepted")
	}
}
