package evolution

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func reservationSpec(request CandidateRequest, env Environment, suite evaluation.FrozenSuite, prepared catalog.PreparedCandidate, config intentConfiguration) (evaluation.ExperimentSpec, error) {
	memoryHash, err := memoryCombinationHash(config.Memories)
	if err != nil {
		return evaluation.ExperimentSpec{}, err
	}
	calls := int(request.Budget.Calls)
	cases := 0
	for _, c := range suite.Cases {
		if c.Partition != "development" {
			cases++
		}
	}
	if cases == 0 || request.Budget.Calls <= 0 || int64(calls) != request.Budget.Calls || request.Budget.Tokens <= 0 || calls < cases*2*suite.Rules.Repeats*minimumCaseCalls(env, len(prepared.Candidate.Nodes)) {
		return evaluation.ExperimentSpec{}, ErrBudget
	}
	spec := evaluation.ExperimentSpec{ID: uuid.NewString(), Kind: "role_change", Purpose: "evaluation", LogicalProjectID: request.ProjectID, EnvironmentID: env.ID, EnvironmentHash: env.Hash, SuiteID: suite.ID, SuiteHash: suite.Hash, PolicyRevision: int(request.PolicyRevision), Generation: 1, BaselineVersion: request.BaselineVersion, CandidateHash: prepared.CandidateHash, SourceBaseline: env.SourceBaseline, ConfigHash: env.ConfigHash, Budget: evaluation.ExperimentBudget{MaxCalls: calls, MaxTokens: request.Budget.Tokens}, Baseline: evaluation.ExperimentArm{RoleHash: prepared.BaselineHash, CompiledHash: prepared.BaselineCompiledHash, MemoryHash: memoryHash}, Candidate: evaluation.ExperimentArm{RoleHash: prepared.CandidateHash, CompiledHash: prepared.CandidateCompiledHash, MemoryHash: memoryHash}}
	if request.Kind == "memory_benefit" {
		if len(config.Memories) != 1 || config.Memories[0].VersionID != request.BenefitMemoryVersionID || prepared.BaselineHash != prepared.CandidateHash || prepared.BaselineCompiledHash != prepared.CandidateCompiledHash {
			return spec, ErrConflict
		}
		spec.Kind = "memory_benefit"
		spec.CandidateHash = config.Memories[0].BodyHash
		spec.Baseline.MemoryHash = evaluation.EmptyMemoryHash
	}
	return spec, nil
}

func minimumCaseCalls(env Environment, nodes int) int {
	if env.Backend == ResearchReplayBackend {
		// The model reads evidence, then uses the tool response to register a report.
		return 2
	}
	return nodes
}

func insertIntentTx(ctx context.Context, tx pgx.Tx, scope Scope, p experience.Policy, intent *intentRecord) error {
	var busy bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2 AND target=ANY($3::text[])) OR EXISTS(SELECT 1 FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2 AND created_at>clock_timestamp()-$4*interval '1 second')`, scope.Namespace, scope.ProjectID, intent.Targets, p.CandidateIntervalSeconds).Scan(&busy)
	if err != nil {
		return err
	}
	if busy {
		return ErrConflict
	}
	return tx.QueryRow(ctx, `INSERT INTO pangu.evolution_intents(id,namespace,project_id,policy_revision,state,request_hash,request,targets) VALUES($1,$2,$3,$4,'preparing',$5,$6,$7) RETURNING created_at`, intent.ID, scope.Namespace, scope.ProjectID, intent.PolicyRevision, intent.RequestHash, jsonBytes(intent.Config), jsonBytes(intent.Targets)).Scan(&intent.StartedAt)
}
