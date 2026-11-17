package evolution

import (
	"github.com/Patrick08T/Pangu/internal/experience"
	"slices"
)

func validatePolicy(policy experience.Policy, request CandidateRequest) error {
	if !policy.Enabled || !policy.AutoPublish || policy.Revoked || policy.ActorID == "" || policy.ProjectID != request.ProjectID || policy.Revision != request.PolicyRevision {
		return ErrForbidden
	}
	if !slices.Contains(policy.Roles, request.AgentObjectID) || !slices.Contains(policy.Environments, request.EnvironmentID) || len(request.Entries) == 0 {
		return ErrForbidden
	}
	if request.FlowObjectID != "" && !slices.Contains(policy.Workflows, request.FlowObjectID) {
		return ErrForbidden
	}
	seen := map[string]bool{}
	for _, entry := range request.Entries {
		if seen[entry.Key()] || !slices.Contains(policy.Entrypoints, entry.Key()) {
			return ErrForbidden
		}
		seen[entry.Key()] = true
	}
	return nil
}
func reserveBudget(limit, used, requested experience.Budget) (experience.Budget, error) {
	if requested.Calls <= 0 || requested.Tokens <= 0 || used.Calls < 0 || used.Tokens < 0 || used.Calls > limit.Calls || used.Tokens > limit.Tokens || requested.Calls > limit.Calls-used.Calls || requested.Tokens > limit.Tokens-used.Tokens {
		return experience.Budget{}, ErrBudget
	}
	return experience.Budget{Calls: used.Calls + requested.Calls, Tokens: used.Tokens + requested.Tokens}, nil
}
