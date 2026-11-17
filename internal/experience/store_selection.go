package experience

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

type selectionResult struct {
	usage    Usage
	versions []Version
}

func (s Store) Select(ctx context.Context, scope Scope, input SelectionInput) (Usage, []Version, error) {
	result, err := withStoreTx(ctx, s, scope, "select", func(tx pgx.Tx, scope Scope) (selectionResult, error) {
		return s.selectTx(ctx, tx, scope, input)
	})
	return result.usage, result.versions, err
}

func (s Store) selectTx(ctx context.Context, tx pgx.Tx, scope Scope, input SelectionInput) (selectionResult, error) {
	if input.ProjectID != scope.ProjectID || input.RunID == "" || input.NodeID == "" || input.Round < 1 || input.Generation < 1 || input.ApprovalRevision < 1 || input.BoundInputHash == "" {
		return selectionResult{}, ErrInvalid
	}
	if input.WatermarkGeneration < 0 {
		return selectionResult{}, ErrInvalid
	}
	if input.WatermarkGeneration == 0 {
		input.WatermarkGeneration = input.Generation
	}
	mark, err := s.readWatermarkTx(ctx, tx, scope, input.RunID, input.WatermarkGeneration)
	if err != nil {
		return selectionResult{}, err
	}
	var policy Policy
	if !s.frozenEmptyOnly {
		policy, err = policyTx(ctx, tx, scope)
		if err != nil {
			return selectionResult{}, err
		}
	}
	var usage Usage
	err = scanJSON(tx.QueryRow(ctx, `SELECT usage FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND identity_hash=$3`, scope.Namespace, scope.ProjectID, selectionIdentity(input)), &usage)
	if err == nil {
		return s.reuseUsageTx(ctx, tx, scope, input, mark, policy, usage)
	}
	if !errors.Is(err, ErrNotFound) {
		return selectionResult{}, err
	}
	if len(mark.Versions) > 0 && (!policy.ReadEnabled || policy.Revoked) {
		return selectionResult{}, ErrForbidden
	}
	versions := []Version{}
	if policyAllowsInput(policy, input) {
		versions, err = s.selectVersionsTx(ctx, tx, scope, input, mark)
		if err != nil {
			return selectionResult{}, err
		}
	}
	return persistUsageTx(ctx, tx, scope, input, mark, versions)
}

func policyAllowsInput(policy Policy, input SelectionInput) bool {
	return policy.ReadEnabled && !policy.Revoked && slices.Contains(policy.Roles, input.RoleID) && authorizedCohort(policy.Delivery, input)
}

func (s Store) selectVersionsTx(ctx context.Context, tx pgx.Tx, scope Scope, input SelectionInput, mark Watermark) ([]Version, error) {
	pool := []Version{}
	for _, frozen := range mark.Versions {
		v, err := s.readVersionTx(ctx, tx, scope, frozen.Reference.VersionID)
		if err == nil {
			err = checkFrozenSafetyTx(ctx, tx, v.ID)
		}
		if errors.Is(err, ErrInvalidated) {
			continue
		}
		if err != nil {
			return nil, err
		}
		pool = append(pool, v)
	}
	return SelectFrozenCandidates(pool, mark, input, time.Now())
}

func (s Store) reuseUsageTx(ctx context.Context, tx pgx.Tx, scope Scope, input SelectionInput, mark Watermark, policy Policy, usage Usage) (selectionResult, error) {
	old := usage.Input
	if usage.WatermarkHash != mark.Hash || selectionIdentity(old) != selectionIdentity(input) || old.RoleID != input.RoleID || old.RoleVersion != input.RoleVersion || old.Module != input.Module || old.CanaryScope != input.CanaryScope {
		return selectionResult{}, ErrConflict
	}
	if usage.InjectionHash != referenceHash(usage.References) {
		return selectionResult{}, ErrInvalidated
	}
	if len(usage.References) > 0 && !policyAllowsInput(policy, input) {
		return selectionResult{}, ErrForbidden
	}
	versions, err := s.resolveReferencesTx(ctx, tx, scope, usage.References)
	return selectionResult{usage: usage, versions: versions}, err
}

func persistUsageTx(ctx context.Context, tx pgx.Tx, scope Scope, input SelectionInput, mark Watermark, versions []Version) (selectionResult, error) {
	input.Text, input.ErrorText = "", ""
	usage := Usage{Input: input, WatermarkHash: mark.Hash, References: []Reference{}, CreatedAt: time.Now().UTC(), MatchReason: "no_reliable_match"}
	for _, version := range versions {
		usage.References = append(usage.References, versionReference(version))
	}
	if len(versions) > 0 {
		usage.MatchReason = "scoped_error_or_keyword_match"
	}
	usage.InjectionHash = referenceHash(usage.References)
	data, err := json.Marshal(usage)
	if err != nil {
		return selectionResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_usage(namespace,project_id,identity_hash,usage) VALUES($1,$2,$3,$4)`, scope.Namespace, scope.ProjectID, selectionIdentity(input), data)
	if err != nil {
		return selectionResult{}, err
	}
	return selectionResult{usage: usage, versions: versions}, nil
}

func referenceHash(refs []Reference) string {
	data, _ := json.Marshal(refs)
	return hashBytes(data)
}
