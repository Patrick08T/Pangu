package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

type evolutionStarter interface {
	StartEvolution(context.Context, evolution.Scope, evolution.CandidateRequest) (*evolution.Intent, error)
}

type evolutionCandidateInput struct {
	Kind                   string                  `json:"kind,omitempty"`
	BenefitMemoryVersionID string                  `json:"benefit_memory_version_id,omitempty"`
	ID                     string                  `json:"id"`
	PolicyRevision         int64                   `json:"policy_revision"`
	BaselineVersion        string                  `json:"baseline_version"`
	EnvironmentID          string                  `json:"environment_id"`
	SuiteID                string                  `json:"suite_id"`
	Instruction            string                  `json:"instruction"`
	FlowVersion            string                  `json:"flow_version,omitempty"`
	BindingIDs             []string                `json:"binding_ids,omitempty"`
	MemoryVersionIDs       []string                `json:"memory_version_ids,omitempty"`
	Entries                []evolution.EntryTarget `json:"entries,omitempty"`
	Budget                 experience.Budget       `json:"budget"`
}

func (h handler) evolutionCandidate(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.evolutionScope(w, r)
	if !ok {
		return
	}
	var input evolutionCandidateInput
	if !decode(w, r, &input) {
		return
	}
	if !input.validKind() {
		problem(w, 400, "角色候选仅可改指令；记忆实验仅可指定单一目标且不得改变指令")
		return
	}
	for _, id := range []string{input.ID, input.BaselineVersion, input.EnvironmentID, input.SuiteID} {
		if _, err := uuid.Parse(id); err != nil {
			problem(w, 400, "候选引用 ID 无效")
			return
		}
	}
	service, ready := h.evolutionRuntime()
	starter, implemented := h.backend.(evolutionStarter)
	if !ready || !implemented {
		problem(w, 503, "独立实验执行器尚未接通")
		return
	}
	request, err := input.candidate(r.Context(), scope)
	if evolutionError(w, err) {
		return
	}
	intent, err := starter.StartEvolution(r.Context(), scope, request)
	if evolutionError(w, err) {
		return
	}
	if intent == nil || intent.ID != request.ID || intent.Generation < 1 || intent.State == "" {
		problem(w, 503, "独立实验尚未建立有效意图")
		return
	}
	result, err := service.ReadIntentSummary(r.Context(), scope, intent.ID)
	if !evolutionError(w, err) {
		write(w, 202, result)
	}
}

func (input evolutionCandidateInput) validKind() bool {
	if input.PolicyRevision < 1 {
		return false
	}
	if input.Kind == "memory_benefit" {
		_, err := uuid.Parse(input.BenefitMemoryVersionID)
		return err == nil && input.Instruction == "" && len(input.MemoryVersionIDs) == 0
	}
	return (input.Kind == "" || input.Kind == "role_change") && input.BenefitMemoryVersionID == "" && strings.TrimSpace(input.Instruction) != "" && len(input.Instruction) <= 65536
}

func (input evolutionCandidateInput) candidate(ctx context.Context, scope evolution.Scope) (evolution.CandidateRequest, error) {
	out := evolution.CandidateRequest{Kind: input.Kind, BenefitMemoryVersionID: input.BenefitMemoryVersionID, ID: input.ID, ProjectID: scope.ProjectID, PolicyRevision: input.PolicyRevision,
		BaselineVersion: input.BaselineVersion, EnvironmentID: input.EnvironmentID, SuiteID: input.SuiteID,
		MemoryVersionIDs: input.MemoryVersionIDs, Entries: input.Entries, Budget: input.Budget}
	version, err := catalog.ReadVersionScoped(ctx, scope.Namespace, input.BaselineVersion, catalog.VersionScope{ProjectID: scope.ProjectID})
	if err != nil {
		return out, err
	}
	if version.Spec.Agent == nil {
		return out, evolution.ErrInvalid
	}
	out.AgentObjectID = version.ObjectID
	out.Payload = catalog.CandidatePayload{Agent: *version.Spec.Agent, FlowVersion: input.FlowVersion, BindingIDs: input.BindingIDs}
	if input.Kind == "memory_benefit" {
		out.MemoryVersionIDs = []string{input.BenefitMemoryVersionID}
	} else {
		out.Payload.Agent.Instruction = input.Instruction
	}
	if input.FlowVersion != "" {
		flow, err := catalog.ReadVersionScoped(ctx, scope.Namespace, input.FlowVersion, catalog.VersionScope{ProjectID: scope.ProjectID})
		if err != nil {
			return out, err
		}
		if flow.Spec.Flow == nil {
			return out, evolution.ErrInvalid
		}
		out.FlowObjectID = flow.ObjectID
	}
	return out, nil
}
