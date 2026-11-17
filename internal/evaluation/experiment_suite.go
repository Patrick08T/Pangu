package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
)

func FreezeSuite(spec SuiteSpec) (*FrozenSuite, error) {
	if err := validateSuite(spec); err != nil {
		return nil, err
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, ErrInvalid
	}
	var copy SuiteSpec
	if err = json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}
	sort.Slice(copy.Cases, func(i, j int) bool { return copy.Cases[i].ID < copy.Cases[j].ID })
	hidden := []string{}
	for _, c := range copy.Cases {
		if c.Partition == "holdout" {
			hidden = append(hidden, c.InputHash)
		}
	}
	sort.Strings(hidden)
	return &FrozenSuite{SuiteSpec: copy, Hash: contentHash(copy), HoldoutHash: contentHash(hidden)}, nil
}

func (s FrozenSuite) Validate() error {
	frozen, err := FreezeSuite(s.SuiteSpec)
	if err != nil {
		return err
	}
	if frozen.Hash != s.Hash || frozen.HoldoutHash != s.HoldoutHash {
		return ErrConflict
	}
	return nil
}

func contentHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validHash(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 32 && hex.EncodeToString(data) == value
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validateRules(r DecisionRules) error {
	if r.Method != "hoeffding_bonferroni" || r.FailurePolicy != "invalidate" {
		return ErrInvalid
	}
	if !finite(r.Confidence) || r.Confidence <= 0 || r.Confidence >= 1 {
		return ErrInvalid
	}
	if r.MinFamilies < 1 || r.Repeats < 1 || r.MaxCandidates < 1 {
		return ErrInvalid
	}
	if !finite(r.MinImprovement) || r.MinImprovement <= 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, m := range r.Metrics {
		if seen[m.Name] || !supportedMetric(m.Name) {
			return ErrInvalid
		}
		if !finite(m.Lower) || !finite(m.Upper) || m.Lower >= m.Upper {
			return ErrInvalid
		}
		if !finite(2*(m.Upper-m.Lower)) || !finite(m.Tolerance) || m.Tolerance < 0 {
			return ErrInvalid
		}
		if m.Name == "acceptance" && (!m.HigherBetter || m.Lower < 0 || m.Upper > 1) {
			return ErrInvalid
		}
		if m.Name != "acceptance" && (m.HigherBetter || m.Lower < 0) {
			return ErrInvalid
		}
		seen[m.Name] = true
	}
	if !seen[r.Primary] || !seen["acceptance"] || !seen["cost"] || !seen["duration"] {
		return ErrInvalid
	}
	groups := map[string]bool{}
	for _, g := range r.Groups {
		if g == "" || groups[g] {
			return ErrInvalid
		}
		groups[g] = true
	}
	return nil
}

func supportedMetric(name string) bool {
	return name == "acceptance" || name == "rework" || name == "cost" || name == "duration"
}
