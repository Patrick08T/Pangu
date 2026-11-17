package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type evolutionMemorySelection struct {
	Usage     experience.Usage
	Watermark experience.Watermark
}

func evolutionBusinessSelectionsTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, run evolutionBusinessRun) (map[string]evolutionMemorySelection, error) {
	rows, err := tx.Query(ctx, `SELECT usage FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND usage#>>'{input,run_id}'=$3 ORDER BY identity_hash`, s.Namespace, s.ProjectID, run.RequestID)
	if err != nil {
		return nil, err
	}
	var usages []experience.Usage
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return nil, err
		}
		var usage experience.Usage
		if json.Unmarshal(data, &usage) != nil {
			rows.Close()
			return nil, evolution.ErrChecker
		}
		usages = append(usages, usage)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	selections := map[string]evolutionMemorySelection{}
	for _, usage := range usages {
		key := fmt.Sprintf("%s/%d", usage.Input.NodeID, usage.Input.Round)
		if _, exists := selections[key]; exists {
			return nil, evolution.ErrChecker
		}
		if err = evolutionBusinessSelectionIdentityTx(ctx, tx, s, run, usage.Input); err != nil {
			return nil, err
		}
		scope := experience.Scope{Namespace: s.Namespace, ProjectID: s.ProjectID, ActorID: run.OwnerID}
		mark, verified, _, readErr := learning.NewStore().ReadFrozenSelectionTx(ctx, tx, scope, usage.Input)
		if readErr != nil {
			return nil, evolutionBusinessMemoryError(readErr)
		}
		if evolutionHash(usage) != evolutionHash(verified) {
			return nil, evolution.ErrChecker
		}
		selections[key] = evolutionMemorySelection{Usage: verified, Watermark: mark}
	}
	if len(selections) == 0 {
		return nil, evolution.ErrChecker
	}
	return selections, nil
}

func evolutionBusinessSelectionIdentityTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, run evolutionBusinessRun, input experience.SelectionInput) error {
	if input.RunID != run.RequestID || input.ProjectID != s.ProjectID || input.Generation != 1 || input.WatermarkGeneration != 1 || input.ApprovalRevision != 1 || input.CanaryScope != run.Operation || len(input.BoundInputHash) != 64 {
		return evolution.ErrChecker
	}
	var node *workflow.Node
	for i := range run.Snapshot.Workflow.Nodes {
		if run.Snapshot.Workflow.Nodes[i].ID == input.NodeID {
			node = &run.Snapshot.Workflow.Nodes[i]
			break
		}
	}
	if node == nil || node.AgentVersion != input.RoleVersion {
		return evolution.ErrChecker
	}
	version, err := catalog.ReadVersionScopedTx(ctx, tx, s.Namespace, node.AgentVersion, catalog.VersionScope{ProjectID: s.ProjectID, AllowObserving: true})
	if err != nil {
		return err
	}
	if version.Kind != "agent" || version.Archived || version.ObjectID != input.RoleID {
		return evolution.ErrChecker
	}
	return nil
}

func evolutionBusinessMemoryError(err error) error {
	for _, expected := range []error{experience.ErrNotFound, experience.ErrInvalid, experience.ErrInvalidated, experience.ErrForbidden, experience.ErrConflict} {
		if errors.Is(err, expected) {
			return evolution.ErrChecker
		}
	}
	return err
}
