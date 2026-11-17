package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
)

// Policy settings deliberately exclude namespace, project, actor and revision.
type experiencePolicySettings struct {
	Enabled                  bool                         `json:"enabled"`
	ReadEnabled              bool                         `json:"read_enabled"`
	AutoPublish              bool                         `json:"auto_publish"`
	Revoked                  bool                         `json:"revoked"`
	Roles                    []string                     `json:"roles"`
	Workflows                []string                     `json:"workflows"`
	Entrypoints              []string                     `json:"entrypoints"`
	ExtractorVersion         string                       `json:"extractor_version"`
	GeneratorVersion         string                       `json:"generator_version"`
	Environments             []string                     `json:"environments"`
	LearningBudget           experience.Budget            `json:"learning_budget"`
	EvaluationBudget         experience.Budget            `json:"evaluation_budget"`
	BudgetPeriodSeconds      int64                        `json:"budget_period_seconds"`
	CandidateIntervalSeconds int64                        `json:"candidate_interval_seconds"`
	Evaluation               experience.EvaluationPolicy  `json:"evaluation"`
	Delivery                 experience.DeliveryPolicy    `json:"delivery"`
	Observation              experience.ObservationPolicy `json:"observation"`
}

type experiencePolicyView struct {
	Revision int64 `json:"revision"`
	experiencePolicySettings
}

func policyView(p experience.Policy) experiencePolicyView {
	return experiencePolicyView{p.Revision, experiencePolicySettings{
		Enabled: p.Enabled, ReadEnabled: p.ReadEnabled, AutoPublish: p.AutoPublish, Revoked: p.Revoked,
		Roles: p.Roles, Workflows: p.Workflows, Entrypoints: p.Entrypoints, ExtractorVersion: p.ExtractorVersion,
		GeneratorVersion: p.GeneratorVersion, Environments: p.Environments, LearningBudget: p.LearningBudget,
		EvaluationBudget: p.EvaluationBudget, BudgetPeriodSeconds: p.BudgetPeriodSeconds,
		CandidateIntervalSeconds: p.CandidateIntervalSeconds, Evaluation: p.Evaluation, Delivery: p.Delivery, Observation: p.Observation,
	}}
}

func (p experiencePolicySettings) policy() experience.Policy {
	return experience.Policy{Enabled: p.Enabled, ReadEnabled: p.ReadEnabled, AutoPublish: p.AutoPublish, Revoked: p.Revoked,
		Roles: p.Roles, Workflows: p.Workflows, Entrypoints: p.Entrypoints, ExtractorVersion: p.ExtractorVersion,
		GeneratorVersion: p.GeneratorVersion, Environments: p.Environments, LearningBudget: p.LearningBudget,
		EvaluationBudget: p.EvaluationBudget, BudgetPeriodSeconds: p.BudgetPeriodSeconds,
		CandidateIntervalSeconds: p.CandidateIntervalSeconds, Evaluation: p.Evaluation, Delivery: p.Delivery, Observation: p.Observation}
}

func (h handler) experiencePolicy(w http.ResponseWriter, r *http.Request) {
	action := "read"
	if r.Method == "PUT" {
		action = "configure"
	}
	scope, ok := h.experienceScope(w, r, action)
	if !ok {
		return
	}
	store := learning.NewStore()
	if r.Method == "GET" {
		result, err := store.Policy(r.Context(), scope)
		if !experienceError(w, err) {
			write(w, 200, policyView(result))
		}
		return
	}
	var input struct {
		ExpectedRevision *int64                    `json:"expected_revision"`
		Policy           *experiencePolicySettings `json:"policy"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ExpectedRevision == nil || input.Policy == nil {
		problem(w, 400, "必须提供 expected_revision 和 policy")
		return
	}
	if (input.Policy.Enabled || input.Policy.AutoPublish) && input.Policy.Evaluation.PrimaryMetric != "acceptance" {
		problem(w, 400, "当前不支持此主指标，独立机器账本仅支持 acceptance")
		return
	}
	if input.Policy.AutoPublish && !h.allowed(w, r, "", "manage_catalog", "") {
		return
	}
	result, err := store.SavePolicy(r.Context(), scope, input.Policy.policy(), *input.ExpectedRevision)
	if !experienceError(w, err) {
		write(w, 200, policyView(result))
	}
}
