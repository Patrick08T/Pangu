package evolution

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type liveExperiment struct {
	Frozen      *evaluation.FrozenExperiment
	Suite       *evaluation.FrozenSuite
	Environment Environment
	Baseline    workflow.Definition
	Candidate   workflow.Definition
	Memories    []workflow.Experience
}

func (s Service) liveExperimentTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, p experience.Policy) (*liveExperiment, error) {
	if err := verifyConfiguredTargetsTx(ctx, tx, scope, intent); err != nil {
		return nil, err
	}
	frozen, err := evaluation.ReadExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, intent.ExperimentID)
	if err != nil {
		return nil, err
	}
	if frozen.Hash != intent.ExperimentHash || frozen.Generation != intent.Generation {
		return nil, ErrConflict
	}
	suite, err := evaluation.ReadExperimentSuiteTx(ctx, tx, scope.Namespace, intent.Config.Request.SuiteID)
	if err != nil {
		return nil, err
	}
	env, err := environmentTx(ctx, tx, scope, intent.Config.Request.EnvironmentID)
	if err != nil {
		return nil, err
	}
	if err = verifySuite(p, suite, checkerHash(env.Backend)); err != nil {
		return nil, err
	}
	memories, err := s.recheckMemoriesTx(ctx, tx, scope, intent.Config)
	if err != nil {
		return nil, err
	}
	prepared, err := liveCompileTx(ctx, tx, scope, intent, suite, env.Backend)
	if err != nil {
		return nil, err
	}
	if err = verifyMemoryScopeTx(ctx, tx, scope, prepared, memories); err != nil {
		return nil, err
	}
	if err = validateTargets(*env, prepared.Baseline, intent.Config.Request); err != nil {
		return nil, err
	}
	if err = ValidateExecution(*env, prepared.Candidate); err != nil {
		return nil, err
	}
	current, err := reservationSpec(intent.Config.Request, *env, *suite, *prepared, intent.Config)
	if err != nil {
		return nil, err
	}
	current.ID = intent.ExperimentID
	current.Generation = intent.Generation
	checked, err := evaluation.FreezeExperiment(suite, current)
	if err != nil {
		return nil, err
	}
	if checked.Hash != frozen.Hash {
		return nil, ErrConflict
	}
	return &liveExperiment{Frozen: checked, Suite: suite, Environment: *env, Baseline: prepared.Baseline, Candidate: prepared.Candidate, Memories: memories}, nil
}

func liveCompileTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, suite *evaluation.FrozenSuite, backend string) (*catalog.PreparedCandidate, error) {
	if intent.Config.Request.Kind == "memory_benefit" {
		return prepareRequestedCandidateTx(ctx, tx, scope, intent.Config.Request)
	}
	return liveRoleCompileTx(ctx, tx, scope, intent, suite, backend)
}

func liveRoleCompileTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, suite *evaluation.FrozenSuite, backend string) (*catalog.PreparedCandidate, error) {
	var caseID string
	for _, c := range suite.Cases {
		if c.Partition != "development" {
			caseID = c.ID
			break
		}
	}
	example, err := evaluation.LoadExperimentCaseTx(ctx, tx, scope.Namespace, intent.ExperimentID, caseID)
	if err != nil {
		return nil, err
	}
	input := example.Input
	if backend == ResearchReplayBackend {
		input, err = replayCaseInput(example.Input, example.Expected, example.Criteria)
		if err != nil {
			return nil, err
		}
	}
	base, identity, err := catalog.CompileExperimentTx(ctx, tx, scope.Namespace, intent.ExperimentID, "baseline", input)
	if err != nil {
		return nil, err
	}
	candidate, _, err := catalog.CompileExperimentTx(ctx, tx, scope.Namespace, intent.ExperimentID, "candidate", input)
	if err != nil {
		return nil, err
	}
	return &catalog.PreparedCandidate{Baseline: *base, Candidate: *candidate, BaselineHash: identity.Baseline.RoleHash, CandidateHash: identity.CandidateHash, BaselineCompiledHash: catalog.CompiledHash(*base), CandidateCompiledHash: catalog.CompiledHash(*candidate)}, nil
}
