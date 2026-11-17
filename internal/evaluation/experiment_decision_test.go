package evaluation

import (
	"math"
	"strings"
	"testing"
)

func pairedReceipts(t *testing.T, s *FrozenSuite, e *FrozenExperiment) []ExperimentReceipt {
	t.Helper()
	out := []ExperimentReceipt{}
	for _, c := range s.Cases {
		if c.Partition == "development" {
			continue
		}
		for repeat := 1; repeat <= s.Rules.Repeats; repeat++ {
			for _, arm := range []string{"baseline", "candidate"} {
				hash, err := ExecutionInputHash(s, e, arm, c.ID, repeat)
				if err != nil {
					t.Fatal(err)
				}
				compiled, value := e.Baseline.CompiledHash, 0.
				if arm == "candidate" {
					compiled, value = e.Candidate.CompiledHash, 1
				}
				out = append(out, ExperimentReceipt{ExperimentID: e.ID, ExperimentHash: e.Hash,
					Generation: e.Generation, Arm: arm, CaseID: c.ID, Repeat: repeat, Attempt: 1,
					ExecutionInputHash: hash, EnvironmentHash: e.EnvironmentHash, CompiledHash: compiled,
					CheckerHash: c.CheckerHash, EvidenceHash: strings.Repeat("9", 64), Status: "completed",
					GatesPassed: true, ProtectionPassed: true, Metrics: map[string]float64{"acceptance": value, "cost": 0, "duration": 0}})
			}
		}
	}
	return out
}

func TestPairedDecisionUsesConservativeFamilyIntervals(t *testing.T) {
	s, e := frozenFixture(t, 200)
	decision := EvaluateExperiment(s, e, pairedReceipts(t, s, e), e.ExperimentSpec)
	if decision.Outcome != "passed" || len(decision.Intervals) != 3 {
		t.Fatalf("%+v", decision)
	}
	i := decision.Intervals[0]
	// Hand checked: paired range [-1,1], alpha=.05/9, n=200 -> radius ~.2426.
	if i.Families != 200 || i.Mean != 1 || i.Lower < .75 || i.Lower > .76 {
		t.Fatalf("wrong independent unit/interval: %+v", i)
	}
}

func TestRepeatedSameFamilyCannotManufactureEvidence(t *testing.T) {
	spec := suiteFixture(1)
	spec.Rules.Repeats = 500
	s, err := FreezeSuite(spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := FreezeExperiment(s, experimentFixture(s))
	if err != nil {
		t.Fatal(err)
	}
	d := EvaluateExperiment(s, e, pairedReceipts(t, s, e), e.ExperimentSpec)
	if d.Outcome == "passed" || len(d.Intervals) != 3 || d.Intervals[0].Families != 1 {
		t.Fatalf("repeat inflation: %+v", d)
	}
}

func TestPairedDecisionRejectsHardFailuresMissingDataAndDrift(t *testing.T) {
	s, e := frozenFixture(t, 200)
	changes := map[string]func([]ExperimentReceipt) []ExperimentReceipt{
		"missing unfavorable sample": func(r []ExperimentReceipt) []ExperimentReceipt { return r[1:] },
		"duplicate retry":            func(r []ExperimentReceipt) []ExperimentReceipt { return append(r, r[0]) },
		"authorization gate":         func(r []ExperimentReceipt) []ExperimentReceipt { r[0].GatesPassed = false; return r },
		"unknown metric":             func(r []ExperimentReceipt) []ExperimentReceipt { delete(r[0].Metrics, "acceptance"); return r },
		"unregistered metric":        func(r []ExperimentReceipt) []ExperimentReceipt { r[0].Metrics["invented_score"] = 1; return r },
		"out of bounds":              func(r []ExperimentReceipt) []ExperimentReceipt { r[0].Metrics["acceptance"] = 2; return r },
		"nonfinite":                  func(r []ExperimentReceipt) []ExperimentReceipt { r[0].Metrics["acceptance"] = math.NaN(); return r },
		"infra error":                func(r []ExperimentReceipt) []ExperimentReceipt { r[0].Status = "infrastructure_error"; return r },
		"timeout":                    func(r []ExperimentReceipt) []ExperimentReceipt { r[0].Status = "timeout"; return r },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			d := EvaluateExperiment(s, e, change(pairedReceipts(t, s, e)), e.ExperimentSpec)
			if d.Outcome == "passed" {
				t.Fatalf("unsafe promotion: %+v", d)
			}
		})
	}
	r := pairedReceipts(t, s, e)
	for i := range r {
		if r[i].CaseID == "case-200" {
			r[i].ProtectionPassed = false
			break
		}
	}
	if d := EvaluateExperiment(s, e, r, e.ExperimentSpec); d.Category != "protection" {
		t.Fatalf("protection averaged away: %+v", d)
	}
	current := e.ExperimentSpec
	current.PolicyRevision++
	if d := EvaluateExperiment(s, e, pairedReceipts(t, s, e), current); d.Category != "drift" {
		t.Fatalf("drift ignored: %+v", d)
	}
}

func TestResourceDirectionAndNonInferiority(t *testing.T) {
	spec := suiteFixture(200)
	spec.Rules.Primary = "cost"
	spec.Rules.Metrics = []MetricRule{{Name: "cost", Lower: 0, Upper: 1, Tolerance: 0},
		{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1, Tolerance: .3},
		{Name: "duration", Lower: 0, Upper: 1, Tolerance: 1}}
	s, err := FreezeSuite(spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := FreezeExperiment(s, experimentFixture(s))
	if err != nil {
		t.Fatal(err)
	}
	r := pairedReceipts(t, s, e)
	for i := range r {
		r[i].Metrics["acceptance"] = 1
		r[i].Metrics["cost"] = 1
		if r[i].Arm == "candidate" {
			r[i].Metrics["cost"] = 0
		}
	}
	if d := EvaluateExperiment(s, e, r, e.ExperimentSpec); d.Outcome != "passed" {
		t.Fatalf("lower cost not recognized: %+v", d)
	}
	for i := range r {
		if r[i].Arm == "candidate" {
			r[i].Metrics["acceptance"] = 0
		}
	}
	if d := EvaluateExperiment(s, e, r, e.ExperimentSpec); d.Category != "non_inferiority" {
		t.Fatalf("quality loss traded away: %+v", d)
	}
}
