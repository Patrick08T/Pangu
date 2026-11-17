package evolution

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s Service) ReserveCandidate(ctx context.Context, scope Scope, request CandidateRequest) (*Intent, error) {
	if _, err := uuid.Parse(request.ID); err != nil {
		return nil, ErrInvalid
	}
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		p, err := policyTx(ctx, tx, scope, request.PolicyRevision)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if err = p.Validate(); err != nil {
			return err
		}
		if err = validatePolicy(p, request); err != nil {
			return err
		}
		prior, err := readIntentTx(ctx, tx, scope, request.ID)
		if err == nil {
			if prior.RequestHash != hashJSON(request) {
				return ErrConflict
			}
			out = &prior.Intent
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		intent, err := s.reserveCandidateTx(ctx, tx, scope, p, request)
		if err != nil {
			return err
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}

func (s Service) reserveCandidateTx(ctx context.Context, tx pgx.Tx, scope Scope, p experience.Policy, request CandidateRequest) (*intentRecord, error) {
	env, err := environmentTx(ctx, tx, scope, request.EnvironmentID)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareRequestedCandidateTx(ctx, tx, scope, request)
	if err != nil {
		return nil, err
	}
	if err = validateExecution(*env, prepared.Baseline); err != nil {
		return nil, err
	}
	if err = validateExecution(*env, prepared.Candidate); err != nil {
		return nil, err
	}
	suite, err := evaluation.ReadExperimentSuiteTx(ctx, tx, scope.Namespace, request.SuiteID)
	if err != nil {
		return nil, err
	}
	if err = verifySuite(p, suite); err != nil {
		return nil, err
	}
	config, err := s.reservationConfigTx(ctx, tx, scope, request, prepared)
	if err != nil {
		return nil, err
	}
	spec, err := reservationSpec(request, *env, *suite, *prepared, config)
	if err != nil {
		return nil, err
	}
	config.Publish.ExperimentID = spec.ID
	intent := &intentRecord{Intent: Intent{ID: request.ID, ProjectID: scope.ProjectID, PolicyRevision: p.Revision, Generation: 1, State: "preparing", RequestHash: hashJSON(request), Targets: targetKeys(request)}, Config: config}
	if err = insertIntentTx(ctx, tx, scope, p, intent); err != nil {
		return nil, err
	}
	if err = reserveClaimsTx(ctx, tx, scope, intent); err != nil {
		return nil, err
	}
	if err = ReserveBudgetTx(ctx, tx, scope, p.Revision, "evaluation", request.ID, request.Budget); err != nil {
		return nil, err
	}
	experiment, err := evaluation.ReserveExperimentTx(ctx, tx, scope.Namespace, spec)
	if err != nil {
		return nil, err
	}
	if err = storeRequestedCandidateTx(ctx, tx, scope, experiment.ID, request); err != nil {
		return nil, err
	}
	intent.ExperimentID, intent.ExperimentHash = experiment.ID, experiment.Hash
	_, err = tx.Exec(ctx, `UPDATE pangu.evolution_intents SET experiment_id=$2,experiment_hash=$3,state='evaluating',updated_at=clock_timestamp() WHERE id=$1`, intent.ID, experiment.ID, experiment.Hash)
	intent.State = "evaluating"
	return intent, err
}

func (s Service) reservationConfigTx(ctx context.Context, tx pgx.Tx, scope Scope, request CandidateRequest, prepared *catalog.PreparedCandidate) (intentConfiguration, error) {
	config := intentConfiguration{Request: request, Publish: catalog.AutomaticPublishRequest{IntentID: request.ID}}
	var err error
	config.Publish.Agent, err = targetSnapshotTx(ctx, tx, scope, request.AgentObjectID, request.BaselineVersion, "agent")
	if err != nil {
		return config, err
	}
	if (request.FlowObjectID == "") != (request.Payload.FlowVersion == "") {
		return config, ErrInvalid
	}
	if request.FlowObjectID != "" {
		target, e := targetSnapshotTx(ctx, tx, scope, request.FlowObjectID, request.Payload.FlowVersion, "workflow")
		if e != nil {
			return config, e
		}
		config.Publish.Flow = &target
	}
	config.Entries, err = s.reservationEntriesTx(ctx, tx, scope, request)
	if err != nil {
		return config, err
	}
	var memories []workflow.Experience
	config.Memories, memories, err = s.freezeMemoriesTx(ctx, tx, scope, request.MemoryVersionIDs)
	if err == nil {
		err = verifyMemoryScopeTx(ctx, tx, scope, prepared, memories)
	}
	// Candidate prompt is private in catalog_experiment_candidates, never duplicated in intent metadata.
	config.Request.Payload.Agent = workflow.Node{}
	return config, err
}
