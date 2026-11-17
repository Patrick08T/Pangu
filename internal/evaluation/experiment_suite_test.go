package evaluation

import (
	"fmt"
	"strings"
	"testing"
)

func suiteFixture(n int) SuiteSpec {
	spec := SuiteSpec{ID: "suite", ProjectID: "project", Version: 1,
		Rules: DecisionRules{Method: "hoeffding_bonferroni", FailurePolicy: "invalidate",
			Confidence: .95, MinFamilies: n, Repeats: 2, MaxCandidates: 3,
			Primary: "acceptance", MinImprovement: .1,
			Metrics: []MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1, Tolerance: 0},
				{Name: "cost", Lower: 0, Upper: 1, Tolerance: 1},
				{Name: "duration", Lower: 0, Upper: 1, Tolerance: 1}}}}
	for i := 0; i < n+2; i++ {
		partition := "holdout"
		if i == n {
			partition = "protection"
		}
		if i == n+1 {
			partition = "development"
		}
		spec.Cases = append(spec.Cases, SuiteCase{ID: fmt.Sprintf("case-%d", i),
			FamilyID: fmt.Sprintf("family-%d", i), TaskID: fmt.Sprintf("task-%d", i),
			SourceRunID: fmt.Sprintf("run-%d", i), InputHash: fmt.Sprintf("%064x", i+1),
			ExpectedHash: strings.Repeat("e", 64), CriteriaHash: strings.Repeat("d", 64), CheckerHash: strings.Repeat("c", 64),
			Partition: partition, Critical: partition == "protection"})
	}
	return spec
}

func TestFreezeSuiteRejectsLeakageAndMissingPreregistration(t *testing.T) {
	if _, err := FreezeSuite(suiteFixture(4)); err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*SuiteSpec){
		"family leakage":           func(s *SuiteSpec) { s.Cases[4].FamilyID = s.Cases[0].FamilyID },
		"task leakage":             func(s *SuiteSpec) { s.Cases[4].TaskID = s.Cases[0].TaskID },
		"source leakage":           func(s *SuiteSpec) { s.Cases[4].SourceRunID = s.Cases[0].SourceRunID },
		"input leakage":            func(s *SuiteSpec) { s.Cases[4].InputHash = s.Cases[0].InputHash },
		"empty holdout":            func(s *SuiteSpec) { s.Cases = s.Cases[4:] },
		"empty protection":         func(s *SuiteSpec) { s.Cases = s.Cases[:4] },
		"missing criteria hash":    func(s *SuiteSpec) { s.Cases[0].CriteriaHash = "" },
		"no critical protection":   func(s *SuiteSpec) { s.Cases[4].Critical = false },
		"missing cost guard":       func(s *SuiteSpec) { s.Rules.Metrics = append(s.Rules.Metrics[:1], s.Rules.Metrics[2:]...) },
		"missing duration guard":   func(s *SuiteSpec) { s.Rules.Metrics = s.Rules.Metrics[:2] },
		"missing quality guard":    func(s *SuiteSpec) { s.Rules.Primary = "cost"; s.Rules.Metrics = s.Rules.Metrics[1:] },
		"missing bounds":           func(s *SuiteSpec) { s.Rules.Metrics[0].Upper = 0 },
		"missing repeats":          func(s *SuiteSpec) { s.Rules.Repeats = 0 },
		"missing samples":          func(s *SuiteSpec) { s.Rules.MinFamilies = 0 },
		"missing candidate budget": func(s *SuiteSpec) { s.Rules.MaxCandidates = 0 },
		"unsupported method":       func(s *SuiteSpec) { s.Rules.Method = "fake_t" },
		"unknown primary":          func(s *SuiteSpec) { s.Rules.Primary = "confidence" },
		"missing improvement":      func(s *SuiteSpec) { s.Rules.MinImprovement = 0 },
		"unbounded retries":        func(s *SuiteSpec) { s.Rules.FailurePolicy = "retry_until_pass" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			spec := suiteFixture(4)
			change(&spec)
			if _, err := FreezeSuite(spec); err == nil {
				t.Fatal("accepted invalid suite")
			}
		})
	}
}

func TestFrozenSuiteDoesNotAliasInputsAndDetectsTampering(t *testing.T) {
	spec := suiteFixture(4)
	frozen, err := FreezeSuite(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Cases[0].ExpectedHash = strings.Repeat("a", 64)
	spec.Rules.Metrics[0].Tolerance = 42
	if frozen.Cases[0].ExpectedHash != strings.Repeat("e", 64) || frozen.Rules.Metrics[0].Tolerance != 0 {
		t.Fatal("frozen suite aliases caller memory")
	}
	if err := frozen.Validate(); err != nil {
		t.Fatal(err)
	}
	frozen.Rules.MinImprovement = .001
	if err := frozen.Validate(); err == nil {
		t.Fatal("modified threshold kept valid signature")
	}
}

func TestHoldoutIdentityIgnoresRenameAndOrder(t *testing.T) {
	a, err := FreezeSuite(suiteFixture(4))
	if err != nil {
		t.Fatal(err)
	}
	spec := suiteFixture(4)
	spec.ID = "renamed"
	spec.Version = 2
	for i := range spec.Cases {
		spec.Cases[i].ID = "renamed-" + spec.Cases[i].ID
	}
	spec.Cases[0], spec.Cases[1] = spec.Cases[1], spec.Cases[0]
	b, err := FreezeSuite(spec)
	if err != nil {
		t.Fatal(err)
	}
	if a.HoldoutHash != b.HoldoutHash {
		t.Fatal("renaming hidden cases reset pool identity")
	}
	if a.Hash == b.Hash {
		t.Fatal("suite identity not included in frozen hash")
	}
}
