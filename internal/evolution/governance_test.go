package evolution

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/experience"
	"testing"
)

func governancePolicy() experience.Policy {
	return experience.Policy{ProjectID: "project", Revision: 3, Enabled: true, AutoPublish: true, ActorID: "actor", Roles: []string{"role"}, Workflows: []string{"flow"}, Entrypoints: []string{"workbench:main"}, Environments: []string{"env"}, EvaluationBudget: experience.Budget{Calls: 10, Tokens: 1000}, BudgetPeriodSeconds: 3600, CandidateIntervalSeconds: 60}
}

func TestGovernanceNeverAcceptsStaleDisabledOrExpandedPolicy(t *testing.T) {
	policy := governancePolicy()
	request := CandidateRequest{ProjectID: "project", PolicyRevision: 3, AgentObjectID: "role", FlowObjectID: "flow", EnvironmentID: "env", Entries: []EntryTarget{{Kind: "workbench", ID: "main"}}}
	if err := validatePolicy(policy, request); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*experience.Policy, *CandidateRequest){
		func(p *experience.Policy, r *CandidateRequest) { p.Enabled = false },
		func(p *experience.Policy, r *CandidateRequest) { p.AutoPublish = false },
		func(p *experience.Policy, r *CandidateRequest) { p.Revoked = true },
		func(p *experience.Policy, r *CandidateRequest) { p.Revision++ },
		func(p *experience.Policy, r *CandidateRequest) { r.AgentObjectID = "other" },
		func(p *experience.Policy, r *CandidateRequest) { r.FlowObjectID = "other" },
		func(p *experience.Policy, r *CandidateRequest) { r.EnvironmentID = "other" },
		func(p *experience.Policy, r *CandidateRequest) {
			r.Entries = []EntryTarget{{Kind: "webhook", ID: "other"}}
		},
		func(p *experience.Policy, r *CandidateRequest) { r.Entries = nil },
	} {
		p := policy
		r := request
		change(&p, &r)
		if !errors.Is(validatePolicy(p, r), ErrForbidden) {
			t.Fatal("授权范围/修订变更未拒绝")
		}
	}
}

func TestBudgetReservationIncludesFailuresAndNeverUsesBusinessDefaults(t *testing.T) {
	limit := experience.Budget{Calls: 10, Tokens: 1000}
	got, err := reserveBudget(limit, experience.Budget{Calls: 7, Tokens: 700}, experience.Budget{Calls: 3, Tokens: 300})
	if err != nil || got.Calls != 10 || got.Tokens != 1000 {
		t.Fatal(got, err)
	}
	for _, request := range []experience.Budget{{Calls: 4, Tokens: 1}, {Calls: 1, Tokens: 301}, {}, {Calls: -1, Tokens: 2}} {
		if _, err = reserveBudget(limit, experience.Budget{Calls: 7, Tokens: 700}, request); !errors.Is(err, ErrBudget) {
			t.Fatal("预算不足或缺失仍被接受")
		}
	}
	if _, err = reserveBudget(experience.Budget{}, experience.Budget{}, experience.Budget{Calls: 1, Tokens: 1}); !errors.Is(err, ErrBudget) {
		t.Fatal("隐式业务预算")
	}
}
