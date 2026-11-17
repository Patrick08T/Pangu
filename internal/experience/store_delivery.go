package experience

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/jackc/pgx/v5"
)

func (s Store) SetDelivery(ctx context.Context, scope Scope, id string, change DeliveryChange) (Version, error) {
	if (change.Policy.Mode == "canary" || change.Policy.Mode == "serving") && s.VerifyBenefit == nil {
		return Version{}, ErrForbidden
	}
	return withStoreTx(ctx, s, scope, "delivery_change", func(tx pgx.Tx, scope Scope) (Version, error) {
		v, err := s.readVersionTx(ctx, tx, scope, id)
		if err != nil {
			return Version{}, err
		}
		if err = checkFrozenSafetyTx(ctx, tx, id); err != nil {
			return Version{}, err
		}
		if change.ExpectedRevision != v.DeliveryRevision {
			return Version{}, ErrConflict
		}
		if !change.Policy.valid() {
			return Version{}, ErrInvalid
		}
		if change.Policy.Mode == "canary" || change.Policy.Mode == "serving" {
			policy, err := policyTx(ctx, tx, scope)
			if err != nil {
				return Version{}, err
			}
			if !deliveryAuthorized(policy, change.Policy) || v.Validity != "active" {
				return Version{}, ErrForbidden
			}
			if change.Policy.Mode == "serving" && v.Delivery != "canary" {
				return Version{}, ErrForbidden
			}
			if err = s.VerifyBenefit(ctx, tx, scope, v, change); err != nil {
				return Version{}, err
			}
		}
		v.DeliveryRevision++
		if err = insertDeliveryTx(ctx, tx, scope, id, v.DeliveryRevision, change.Policy, change.EvidenceID); err != nil {
			return Version{}, err
		}
		if err = auditTx(ctx, tx, scope, id, change); err != nil {
			return Version{}, err
		}
		v.Delivery, v.Canary = change.Policy.Mode, change.Policy
		return v, nil
	})
}

func deliveryAuthorized(policy Policy, delivery DeliveryPolicy) bool {
	if !policy.Enabled || policy.Revoked || !policy.ReadEnabled || !policy.Delivery.valid() {
		return false
	}
	if policy.Delivery.Mode != "serving" && policy.Delivery.Mode != delivery.Mode {
		return false
	}
	if delivery.TrafficBasisPoints > policy.Delivery.TrafficBasisPoints {
		return false
	}
	for _, scope := range delivery.TaskScope {
		if !slices.Contains(policy.Delivery.TaskScope, scope) {
			return false
		}
	}
	return true
}

func insertDeliveryTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, revision int64, policy DeliveryPolicy, evidence string) error {
	data, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,evidence_id,actor_id)
	 VALUES($1,$2,$3,$4,$5,$6)`, id, revision, policy.Mode, data, evidence, scope.ActorID)
	return err
}

func (s Store) Revoke(ctx context.Context, scope Scope, id, reason string) error {
	_, err := withStoreTx(ctx, s, scope, "version_revoke", func(tx pgx.Tx, scope Scope) (bool, error) {
		if reason == "" || len(reason) > 512 || diagnostic.Sensitive([]byte(reason)) {
			return false, ErrInvalid
		}
		result, err := tx.Exec(ctx, `UPDATE pangu.experience_states s SET validity='revoked',tombstoned=true,invalid_reason=$4,updated_at=now()
		 FROM pangu.experience_versions v WHERE v.id=s.version_id AND v.namespace=$1 AND v.project_id=$2 AND v.id=$3`, scope.Namespace, scope.ProjectID, id, reason)
		if err != nil {
			return false, err
		}
		if result.RowsAffected() != 1 {
			return false, ErrNotFound
		}
		if _, err = tx.Exec(ctx, `DELETE FROM pangu.experience_bodies WHERE version_id=$1`, id); err != nil {
			return false, err
		}
		return true, auditTx(ctx, tx, scope, id, map[string]string{"reason": reason})
	})
	return err
}
