package experience

import (
	"math"
	"slices"
	"strings"
)

// Policy 保存显式授权，不从业务任务继承预算或授权范围。
type Policy struct {
	ProjectID                string            `json:"project_id"`
	Revision                 int64             `json:"revision"`
	ActorID                  string            `json:"actor_id"`
	Enabled                  bool              `json:"enabled"`
	ReadEnabled              bool              `json:"read_enabled"`
	AutoPublish              bool              `json:"auto_publish"`
	Revoked                  bool              `json:"revoked"`
	Roles                    []string          `json:"roles"`
	Workflows                []string          `json:"workflows"`
	Entrypoints              []string          `json:"entrypoints"`
	ExtractorVersion         string            `json:"extractor_version"`
	GeneratorVersion         string            `json:"generator_version"`
	Environments             []string          `json:"environments"`
	LearningBudget           Budget            `json:"learning_budget"`
	EvaluationBudget         Budget            `json:"evaluation_budget"`
	BudgetPeriodSeconds      int64             `json:"budget_period_seconds"`
	CandidateIntervalSeconds int64             `json:"candidate_interval_seconds"`
	Evaluation               EvaluationPolicy  `json:"evaluation"`
	Delivery                 DeliveryPolicy    `json:"delivery"`
	Observation              ObservationPolicy `json:"observation"`
}

type Budget struct {
	Calls  int64 `json:"calls"`
	Tokens int64 `json:"tokens"`
}

type EvaluationPolicy struct {
	SuiteVersion       string   `json:"suite_version"`
	ProtectionSet      string   `json:"protection_set"`
	HoldoutSet         string   `json:"holdout_set"`
	Grouping           string   `json:"grouping"`
	Repeats            int      `json:"repeats"`
	MinimumSamples     int      `json:"minimum_samples"`
	PrimaryMetric      string   `json:"primary_metric"`
	MinimumImprovement float64  `json:"minimum_improvement"`
	NonInferiority     *float64 `json:"non_inferiority"`
	UncertaintyMethod  string   `json:"uncertainty_method"`
	Confidence         float64  `json:"confidence"`
	MaxCandidates      int      `json:"max_candidates"`
	MultipleComparison string   `json:"multiple_comparison"`
	ResourceTolerance  *float64 `json:"resource_tolerance"`
}

type DeliveryPolicy struct {
	Mode               string   `json:"mode"`
	TaskScope          []string `json:"task_scope"`
	TrafficBasisPoints int      `json:"traffic_basis_points"`
}

type ObservationPolicy struct {
	WindowSeconds             int64   `json:"window_seconds"`
	MinimumSamples            int     `json:"minimum_samples"`
	DeadlineSeconds           int64   `json:"deadline_seconds"`
	DegradationThreshold      float64 `json:"degradation_threshold"`
	RetentionSeconds          int64   `json:"retention_seconds"`
	SourceRetentionAuthorized bool    `json:"source_retention_authorized"`
}

func (p Policy) Validate() error {
	if !p.Enabled && !p.AutoPublish {
		return nil
	}
	if p.Revoked || !p.Enabled || p.ProjectID == "" || p.ActorID == "" || p.Revision < 1 ||
		!nonempty(p.Roles) || !nonempty(p.Environments) || p.ExtractorVersion == "" ||
		p.GeneratorVersion == "" || p.LearningBudget.Calls <= 0 || p.LearningBudget.Tokens <= 0 ||
		p.EvaluationBudget.Calls <= 0 || p.EvaluationBudget.Tokens <= 0 ||
		p.BudgetPeriodSeconds <= 0 || p.CandidateIntervalSeconds <= 0 {
		return ErrInvalid
	}
	if p.AutoPublish && (!nonempty(p.Workflows) || !nonempty(p.Entrypoints)) {
		return ErrInvalid
	}
	if !p.Evaluation.valid() || !p.Observation.valid() || !p.Delivery.valid() {
		return ErrInvalid
	}
	return nil
}

func nonempty(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func (e EvaluationPolicy) valid() bool {
	return e.SuiteVersion != "" && e.ProtectionSet != "" && e.HoldoutSet != "" &&
		e.ProtectionSet != e.HoldoutSet && e.Grouping != "" && e.Repeats > 0 && e.MinimumSamples > 0 &&
		slices.Contains([]string{"acceptance", "rework", "cost", "duration"}, e.PrimaryMetric) &&
		positive(e.MinimumImprovement) && e.NonInferiority != nil && nonnegative(*e.NonInferiority) &&
		e.UncertaintyMethod != "" && positive(e.Confidence) && e.Confidence < 1 &&
		e.MaxCandidates > 0 && e.MultipleComparison != "" && e.ResourceTolerance != nil && nonnegative(*e.ResourceTolerance)
}

func (o ObservationPolicy) valid() bool {
	return o.WindowSeconds > 0 && o.MinimumSamples > 0 && o.DeadlineSeconds >= o.WindowSeconds &&
		positive(o.DegradationThreshold) && o.RetentionSeconds > 0 && o.SourceRetentionAuthorized
}

func (d DeliveryPolicy) valid() bool {
	if d.Mode == "shadow" || d.Mode == "paused" {
		return true
	}
	return (d.Mode == "canary" || d.Mode == "serving") && nonempty(d.TaskScope) &&
		d.TrafficBasisPoints > 0 && d.TrafficBasisPoints <= 10000
}

func nonnegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
func positive(value float64) bool { return nonnegative(value) && value > 0 }
