package evolution

import (
	"context"
	"reflect"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func prepareRequestedCandidateTx(ctx context.Context, tx pgx.Tx, scope Scope, request CandidateRequest) (*catalog.PreparedCandidate, error) {
	if request.Kind != "" && request.Kind != "role_change" && request.Kind != "memory_benefit" {
		return nil, ErrInvalid
	}
	payload := request.Payload
	if request.Kind == "memory_benefit" {
		if request.BenefitMemoryVersionID == "" || len(request.MemoryVersionIDs) != 1 || request.MemoryVersionIDs[0] != request.BenefitMemoryVersionID {
			return nil, ErrInvalid
		}
		base, err := catalog.ReadVersionScopedTx(ctx, tx, scope.Namespace, request.BaselineVersion, catalog.VersionScope{ProjectID: scope.ProjectID})
		if err != nil {
			return nil, err
		}
		if base.Spec.Agent == nil {
			return nil, ErrInvalid
		}
		if !reflect.DeepEqual(payload.Agent, workflow.Node{}) && !reflect.DeepEqual(payload.Agent, *base.Spec.Agent) {
			return nil, ErrForbidden
		}
		payload.Agent = *base.Spec.Agent
	} else if request.BenefitMemoryVersionID != "" {
		return nil, ErrInvalid
	}
	return catalog.PrepareExperimentCandidateTx(ctx, tx, scope.Namespace, scope.ProjectID, request.BaselineVersion, payload)
}

func storeRequestedCandidateTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, request CandidateRequest) error {
	if request.Kind == "memory_benefit" {
		return nil
	}
	return catalog.StoreExperimentCandidateTx(ctx, tx, scope.Namespace, id, request.Payload)
}

func verifyConfiguredTargetsTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	request := intent.Config.Request
	agent, err := targetSnapshotTx(ctx, tx, scope, request.AgentObjectID, request.BaselineVersion, "agent")
	if err != nil {
		return err
	}
	if agent != intent.Config.Publish.Agent {
		return ErrConflict
	}
	if request.FlowObjectID != "" {
		flow, err := targetSnapshotTx(ctx, tx, scope, request.FlowObjectID, request.Payload.FlowVersion, "workflow")
		if err != nil {
			return err
		}
		if intent.Config.Publish.Flow == nil || flow != *intent.Config.Publish.Flow {
			return ErrConflict
		}
	}
	return nil
}
