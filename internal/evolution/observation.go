package evolution

import (
	"github.com/Patrick08T/Pangu/internal/experience"
	"time"
)

type Binding struct {
	EntryKey       string `json:"entry_key"`
	Revision       int64  `json:"revision"`
	AdoptedVersion string `json:"adopted_version"`
	StableVersion  string `json:"stable_version"`
	IntentID       string `json:"intent_id"`
	Generation     int    `json:"generation"`
}

type Observation struct {
	Delta       float64 `json:"delta"`
	HardFailure bool    `json:"hard_failure"`
}

func observationOutcome(policy experience.ObservationPolicy, start, now time.Time, samples []Observation) (string, error) {
	if policy.MinimumSamples < 1 || policy.WindowSeconds < 1 || policy.DeadlineSeconds < policy.WindowSeconds || policy.DegradationThreshold <= 0 || start.IsZero() || now.Before(start) {
		return "", ErrInvalid
	}
	total := 0.0
	for _, sample := range samples {
		if sample.HardFailure {
			return "rollback", nil
		}
		total += sample.Delta
	}
	if len(samples) >= policy.MinimumSamples && total/float64(len(samples)) < -policy.DegradationThreshold {
		return "rollback", nil
	}
	age := now.Sub(start)
	if len(samples) >= policy.MinimumSamples && age >= time.Duration(policy.WindowSeconds)*time.Second {
		return "stable", nil
	}
	if age >= time.Duration(policy.DeadlineSeconds)*time.Second {
		return "insufficient", nil
	}
	return "observing", nil
}
func adoptBinding(current, expected Binding, intent string, generation int, next string) (Binding, error) {
	if current != expected || current.IntentID != "" || intent == "" || generation < 1 || next == "" || current.StableVersion == "" {
		return Binding{}, ErrConflict
	}
	current.AdoptedVersion = next
	current.IntentID = intent
	current.Generation = generation
	current.Revision++
	return current, nil
}
func finishBinding(current Binding, intent string, generation int, stable bool) (Binding, error) {
	if intent == "" || current.IntentID != intent || current.Generation != generation || current.StableVersion == "" {
		return Binding{}, ErrConflict
	}
	if stable {
		current.StableVersion = current.AdoptedVersion
	} else {
		current.AdoptedVersion = current.StableVersion
	}
	current.IntentID = ""
	current.Generation = 0
	current.Revision++
	return current, nil
}
