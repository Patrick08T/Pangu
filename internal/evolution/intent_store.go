package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type intentConfiguration struct {
	Request  CandidateRequest                `json:"request"`
	Publish  catalog.AutomaticPublishRequest `json:"publish"`
	Entries  []EntrySnapshot                 `json:"entries"`
	Memories []workflow.MemoryReference      `json:"memories"`
}

type intentRecord struct {
	Intent
	Config intentConfiguration
}

func readIntentTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (*intentRecord, error) {
	var out intentRecord
	var config, targets, publication []byte
	err := tx.QueryRow(ctx, `SELECT id::text,project_id::text,policy_revision,generation,state,request_hash,coalesce(experiment_id::text,''),coalesce(experiment_hash,''),targets,publication,coalesce(observing_at,created_at),request FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, scope.Namespace, scope.ProjectID, id).Scan(&out.ID, &out.ProjectID, &out.PolicyRevision, &out.Generation, &out.State, &out.RequestHash, &out.ExperimentID, &out.ExperimentHash, &targets, &publication, &out.StartedAt, &config)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(config, &out.Config); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(targets, &out.Targets); err != nil {
		return nil, err
	}
	if len(publication) > 0 {
		if err = json.Unmarshal(publication, &out.Publication); err != nil {
			return nil, err
		}
	}
	return &out, nil
}

func authorizedIntentTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) (*intentRecord, experience.Policy, error) {
	var p experience.Policy
	if err := lockScope(ctx, tx, scope); err != nil {
		return nil, p, err
	}
	intent, err := readIntentTx(ctx, tx, scope, id)
	if err != nil {
		return nil, p, err
	}
	p, err = policyTx(ctx, tx, scope, intent.PolicyRevision)
	if err != nil {
		return nil, p, err
	}
	if generation != intent.Generation {
		return nil, p, ErrConflict
	}
	if err = p.Validate(); err != nil {
		return nil, p, ErrForbidden
	}
	if err = validatePolicy(p, intent.Config.Request); err != nil {
		return nil, p, err
	}
	// Completed benefit evidence is read-only; every caller still gates its operation by state.
	if intent.State == "stable" && intent.Config.Request.Kind == "memory_benefit" {
		return intent, p, nil
	}
	if err = verifyClaimsTx(ctx, tx, scope, intent); err != nil {
		return nil, p, err
	}
	return intent, p, nil
}

func verifyClaimsTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	rows, err := tx.Query(ctx, `SELECT target FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2 AND intent_id=$3 AND generation=$4 ORDER BY target FOR UPDATE`, scope.Namespace, scope.ProjectID, intent.ID, intent.Generation)
	if err != nil {
		return err
	}
	defer rows.Close()
	actual := []string{}
	for rows.Next() {
		var target string
		if err = rows.Scan(&target); err != nil {
			return err
		}
		actual = append(actual, target)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if !slices.Equal(actual, intent.Targets) {
		return ErrConflict
	}
	return nil
}

func setIntentStateTx(ctx context.Context, tx pgx.Tx, intent *intentRecord, state string) error {
	tag, err := tx.Exec(ctx, `UPDATE pangu.evolution_intents SET state=$3,updated_at=clock_timestamp() WHERE id=$1 AND generation=$2 AND state=$4`, intent.ID, intent.Generation, state, intent.State)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	intent.State = state
	return nil
}
