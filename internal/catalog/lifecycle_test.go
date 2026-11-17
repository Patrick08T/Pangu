package catalog

import (
	"errors"
	"testing"
)

func TestAutomaticLifecycleIsOneWayAndRequiresLiveBody(t *testing.T) {
	tests := []struct {
		from, to    string
		cleared, ok bool
	}{
		{"observing", "stable", false, true}, {"observing", "revoked", false, true}, {"stable", "revoked", false, true},
		{"stable", "observing", false, false}, {"revoked", "stable", false, false}, {"revoked", "observing", false, false},
		{"observing", "observing", false, false}, {"unknown", "stable", false, false}, {"observing", "stable", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.from+"_"+tt.to, func(t *testing.T) {
			err := validateAutomaticTransition(Version{SourceKind: "auto", State: tt.from, Tombstoned: tt.cleared}, tt.to)
			if tt.ok && err != nil {
				t.Fatal(err)
			}
			if !tt.ok && !errors.Is(err, ErrConflict) {
				t.Fatalf("illegal transition accepted: %v", err)
			}
		})
	}
	if err := validateAutomaticTransition(Version{SourceKind: "manual", State: "observing"}, "stable"); !errors.Is(err, ErrConflict) {
		t.Fatal("manual version accepted")
	}
}
