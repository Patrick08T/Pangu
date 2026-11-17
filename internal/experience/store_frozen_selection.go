package experience

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ReadFrozenSelectionTx never creates a usage or reranks candidates. On a missing
// usage it returns ErrNotFound and the verified watermark, not an empty selection.
func (s Store) ReadFrozenSelectionTx(ctx context.Context, tx pgx.Tx, scope Scope, input SelectionInput) (Watermark, Usage, []Version, error) {
	scope.Action = "select"
	if err := s.authorizeTx(ctx, tx, scope); err != nil {
		return Watermark{}, Usage{}, nil, err
	}
	if input.ProjectID != scope.ProjectID || input.RunID == "" || input.NodeID == "" || input.Round < 1 || input.Generation < 1 || input.ApprovalRevision < 1 || !validHash(input.BoundInputHash) || input.WatermarkGeneration < 0 {
		return Watermark{}, Usage{}, nil, ErrInvalid
	}
	if input.WatermarkGeneration == 0 {
		input.WatermarkGeneration = input.Generation
	}
	mark, err := s.readWatermarkTx(ctx, tx, scope, input.RunID, input.WatermarkGeneration)
	if err != nil {
		return Watermark{}, Usage{}, nil, err
	}
	var usage Usage
	err = scanJSON(tx.QueryRow(ctx, `SELECT usage FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND identity_hash=$3`, scope.Namespace, scope.ProjectID, selectionIdentity(input)), &usage)
	if err != nil {
		return mark, Usage{}, nil, err
	}
	if err = frozenUsageMembership(mark, usage); err != nil {
		return mark, Usage{}, nil, err
	}
	var policy Policy
	if !s.frozenEmptyOnly {
		policy, err = policyTx(ctx, tx, scope)
		if err != nil {
			return mark, Usage{}, nil, err
		}
	}
	result, err := s.reuseUsageTx(ctx, tx, scope, input, mark, policy, usage)
	if err != nil {
		return mark, Usage{}, nil, err
	}
	if err = s.Authorize(ctx, tx, scope); err != nil {
		return mark, Usage{}, nil, err
	}
	return mark, result.usage, result.versions, nil
}

func frozenUsageMembership(mark Watermark, usage Usage) error {
	generation := usage.Input.WatermarkGeneration
	if generation == 0 {
		generation = usage.Input.Generation
	}
	if generation != mark.Generation {
		return ErrInvalidated
	}
	for _, ref := range usage.References {
		found := false
		for _, frozen := range mark.Versions {
			if frozen.Reference == ref {
				found = true
				break
			}
		}
		if !found {
			return ErrInvalidated
		}
	}
	return nil
}
