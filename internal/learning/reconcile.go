package learning

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// CheckMemoryObservation checks live evidence and persists a per-version pause when required.
func CheckMemoryObservation(ctx context.Context, scope experience.Scope, versionID string, reader BenefitPlanReader) (bool, error) {
	return transaction(ctx, scope, "delivery_change", func(tx pgx.Tx, scope experience.Scope) (bool, error) {
		var mode, evidence string
		var revision int64
		err := tx.QueryRow(ctx, `SELECT d.mode,d.revision,d.evidence_id FROM pangu.experience_versions v JOIN LATERAL
   (SELECT mode,revision,evidence_id FROM pangu.experience_deliveries WHERE version_id=v.id ORDER BY revision DESC LIMIT 1) d ON true
   WHERE v.id=$1 AND v.namespace=$2 AND v.project_id=$3`, versionID, scope.Namespace, scope.ProjectID).Scan(&mode, &revision, &evidence)
		if err != nil {
			return false, err
		}
		if mode != "canary" && mode != "serving" {
			return false, nil
		}
		disabled, err := memoryPolicyDisabled(ctx, tx, scope)
		if err != nil {
			return false, err
		}
		if disabled {
			return pauseMemoryTx(ctx, tx, scope, versionID, revision, "policy_disabled")
		}
		version, err := NewStore().ReadTx(ctx, tx, scope, versionID)
		if err != nil {
			if !memoryGuardFailure(err) {
				return false, err
			}
			return pauseMemoryTx(ctx, tx, scope, versionID, revision, "live_evidence_invalid")
		}
		if reader == nil {
			return pauseMemoryTx(ctx, tx, scope, versionID, revision, "missing_plan_reader")
		}
		change := experience.DeliveryChange{EvidenceID: evidence, Policy: experience.DeliveryPolicy{Mode: "canary"}}
		if err = verifyBenefitWithPlan(ctx, tx, scope, version, change, reader); err != nil {
			if !memoryGuardFailure(err) {
				return false, err
			}
			return pauseMemoryTx(ctx, tx, scope, versionID, revision, "benefit_guard_invalid")
		}
		return reconcileMemoryWindow(ctx, tx, scope, version, evidence)
	})
}

func memoryPolicyDisabled(ctx context.Context, tx pgx.Tx, scope experience.Scope) (bool, error) {
	var disabled bool
	err := tx.QueryRow(ctx, `SELECT NOT coalesce((policy->>'enabled')::boolean,false) OR NOT coalesce((policy->>'read_enabled')::boolean,false)
  FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, scope.Namespace, scope.ProjectID).Scan(&disabled)
	return disabled, err
}

func pauseMemoryTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string, revision int64, reason string) (bool, error) {
	policy := map[string]any{"mode": "paused"}
	if reason == "live_evidence_invalid" || reason == "benefit_guard_invalid" || reason == "confirmed_degradation" || reason == "observation_source_invalid" {
		policy["block_frozen"] = true
	}
	data, _ := json.Marshal(policy)
	_, err := tx.Exec(ctx, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,$2,'paused',$3,$4)`, id, revision+1, data, scope.ActorID)
	if err != nil {
		return false, err
	}
	scope.Action = "memory_pause"
	if err = auditTx(ctx, tx, scope, id, map[string]any{"previous_revision": revision, "reason": reason}); err != nil {
		return false, err
	}
	return true, nil
}

// ReconcileMemory checks the namespace's live deliveries as their current policy authors.
func ReconcileMemory(ctx context.Context, namespace string, reader BenefitPlanReader) (int, error) {
	if namespace == "" {
		return 0, experience.ErrForbidden
	}
	conn, err := database.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	rows, err := conn.Query(ctx, `SELECT p.project_id::text,p.actor_id,v.id::text FROM
 (SELECT DISTINCT ON(project_id) project_id,actor_id FROM pangu.experience_policies WHERE namespace=$1 ORDER BY project_id,revision DESC) p
 JOIN pangu.experience_versions v ON v.project_id=p.project_id AND v.namespace=$1
 JOIN LATERAL(SELECT mode FROM pangu.experience_deliveries WHERE version_id=v.id ORDER BY revision DESC LIMIT 1) d ON d.mode IN ('canary','serving')`, namespace)
	if err != nil {
		return 0, err
	}
	type target struct{ project, actor, id string }
	targets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (target, error) {
		var out target
		err := row.Scan(&out.project, &out.actor, &out.id)
		return out, err
	})
	if err != nil {
		return 0, err
	}
	count := 0
	var failures error
	for _, item := range targets {
		paused, err := CheckMemoryObservation(ctx, experience.Scope{Namespace: namespace, ProjectID: item.project, ActorID: item.actor}, item.id, reader)
		if err != nil {
			failures = errors.Join(failures, err)
		}
		if paused {
			count++
		}
	}
	return count, failures
}

func reconcileMemoryWindow(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, evidence string) (bool, error) {
	return memoryWindowAssessment(ctx, tx, scope, version, evidence)
}
