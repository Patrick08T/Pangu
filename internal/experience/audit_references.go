package experience

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

func validAudit() AuditState { return AuditState{Status: "valid", Proven: true} }
func invalidAudit(reason string) AuditState {
	return AuditState{Status: "invalid", Proven: true, InvalidReason: reason}
}
func unprovenAudit(reason string) AuditState {
	return AuditState{Status: "unproven", InvalidReason: reason}
}
func restrictedAudit() AuditState {
	return AuditState{Status: "restricted", InvalidReason: "source_unavailable", Redacted: true}
}

func mergeAudit(a, b AuditState) AuditState {
	rank := map[string]int{"valid": 0, "unproven": 1, "invalid": 2, "restricted": 3}
	result := a
	if rank[b.Status] > rank[a.Status] {
		result = b
	}
	result.Redacted = a.Redacted || b.Redacted
	result.Proven = a.Proven && b.Proven
	return result
}

func (s Store) auditReferencesTx(ctx context.Context, tx pgx.Tx, scope Scope, refs []Reference) (AuditState, error) {
	state := validAudit()
	if len(refs) > 0 && s.VerifySource == nil {
		return state, ErrForbidden
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		if seen[ref.VersionID] {
			state = mergeAudit(state, unprovenAudit("reference_corrupt"))
			continue
		}
		seen[ref.VersionID] = true
		current, err := s.auditReferenceTx(ctx, tx, scope, ref)
		if err != nil {
			return AuditState{}, err
		}
		state = mergeAudit(state, current)
	}
	if len(refs) > 0 {
		policy, err := policyTx(ctx, tx, scope)
		if err != nil {
			return AuditState{}, err
		}
		if !policy.ReadEnabled || policy.Revoked {
			state = mergeAudit(state, invalidAudit("policy_read_revoked"))
		}
	}
	return state, nil
}

func (s Store) auditReferenceTx(ctx context.Context, tx pgx.Tx, scope Scope, ref Reference) (AuditState, error) {
	if !uuidValue(ref.VersionID) || !validHash(ref.BodyHash) || ref.DeliveryRevision < 1 {
		return redactedUnproven("reference_corrupt"), nil
	}
	var hash, validity string
	var sources []byte
	var expires time.Time
	var tombstone, bodyPresent, deliveryPresent bool
	err := tx.QueryRow(ctx, `SELECT v.body_hash,v.sources,v.expires_at,s.validity,s.tombstoned,
 EXISTS(SELECT 1 FROM pangu.experience_bodies b WHERE b.version_id=v.id),
 EXISTS(SELECT 1 FROM pangu.experience_deliveries d WHERE d.version_id=v.id AND d.revision=$4 AND d.mode IN ('canary','serving'))
 FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id
 WHERE v.namespace=$1 AND v.project_id=$2 AND v.id=$3`, scope.Namespace, scope.ProjectID, ref.VersionID, ref.DeliveryRevision).Scan(&hash, &sources, &expires, &validity, &tombstone, &bodyPresent, &deliveryPresent)
	if errors.Is(err, pgx.ErrNoRows) {
		state := unprovenAudit("experience_missing")
		state.Redacted = true
		return state, nil
	}
	if err != nil {
		return AuditState{}, err
	}
	var originals []Source
	if err = json.Unmarshal(sources, &originals); err != nil {
		return AuditState{}, err
	}
	state := validAudit()
	for _, source := range originals {
		if source.ProjectID != scope.ProjectID {
			return restrictedAudit(), nil
		}
		current, err := sourceAuditState(s.VerifySource(ctx, tx, scope, source))
		if err != nil {
			return AuditState{}, err
		}
		state = mergeAudit(state, current)
	}
	if state.Redacted {
		return state, nil
	}
	if len(originals) == 0 || hash != ref.BodyHash || !deliveryPresent {
		return redactedUnproven("reference_corrupt"), nil
	}
	state = referenceValidity(validity, expires, tombstone, bodyPresent)
	if state.Status != "valid" {
		return state, nil
	}
	blocked, err := frozenSafetyBlockedTx(ctx, tx, ref.VersionID)
	if err != nil {
		return AuditState{}, err
	}
	if blocked {
		return invalidAudit("frozen_safety_blocked"), nil
	}
	return state, nil
}

func referenceValidity(validity string, expires time.Time, tombstone, bodyPresent bool) AuditState {
	if tombstone || validity == "revoked" {
		return invalidAudit("experience_revoked")
	}
	if validity == "expired" || !expires.After(time.Now()) {
		return invalidAudit("experience_expired")
	}
	if !bodyPresent {
		return invalidAudit("body_erased")
	}
	if validity != "active" && validity != "superseded" {
		return invalidAudit("experience_inactive")
	}
	return validAudit()
}

func (s Store) auditWatermarkTx(ctx context.Context, tx pgx.Tx, scope Scope, b RunBinding) (WatermarkSummary, error) {
	result := WatermarkSummary{Generation: b.WatermarkGeneration, References: []Reference{}}
	mark, err := watermarkTx(ctx, tx, scope, b.RequestID, b.WatermarkGeneration)
	if errors.Is(err, ErrNotFound) {
		result.AuditState = unprovenAudit("watermark_missing")
		return result, nil
	}
	if errors.Is(err, ErrInvalidated) {
		result.Present = true
		result.AuditState = unprovenAudit("watermark_corrupt")
		return result, nil
	}
	if err != nil {
		return WatermarkSummary{}, err
	}
	result.Present, result.PolicyRevision, result.Hash = true, mark.PolicyRevision, mark.Hash
	for _, item := range mark.Versions {
		result.References = append(result.References, item.Reference)
	}
	result.AuditState, err = s.auditReferencesTx(ctx, tx, scope, result.References)
	if result.Redacted {
		result.Hash = ""
		result.References = []Reference{}
	}
	return result, err
}
