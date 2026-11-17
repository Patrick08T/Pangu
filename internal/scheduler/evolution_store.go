package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

type evolutionStoredPlan struct {
	Plan            evolution.ExecutionPlan `json:"plan"`
	EnvironmentHash string                  `json:"environment_hash"`
}

func evolutionJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }
func evolutionTextHash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}
func evolutionHash(v any) string { return evolutionTextHash(string(evolutionJSON(v))) }
func evolutionEnvelope(p evolution.ExecutionPlan) evolutionStoredPlan {
	return evolutionStoredPlan{Plan: p, EnvironmentHash: p.Environment.Hash}
}

func evolutionTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func evolutionFence(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan) error {
	if err := identity.Lock(ctx, tx, s.Namespace); err != nil {
		return err
	}
	if err := experience.Lock(ctx, tx, s.Namespace, s.ProjectID); err != nil {
		return err
	}
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_intents i JOIN pangu.users u ON u.id=$4 AND u.namespace=i.namespace AND NOT u.disabled
 JOIN LATERAL(SELECT * FROM pangu.experience_policies WHERE namespace=i.namespace AND project_id=i.project_id ORDER BY revision DESC LIMIT 1) p ON p.revision=i.policy_revision
 JOIN LATERAL(SELECT * FROM pangu.evolution_environments WHERE namespace=i.namespace AND project_id=i.project_id AND id=$8 ORDER BY revision DESC LIMIT 1) e ON e.spec_hash=$9
 WHERE i.namespace=$1 AND i.project_id=$2 AND i.id=$3 AND i.generation=$5 AND i.state='evaluating' AND i.experiment_hash=$6 AND i.policy_revision=$7 AND p.actor_id=u.id::text
 AND p.policy->>'enabled'='true' AND p.policy->>'revoked'<>'true' AND e.spec->>'enabled'='true'
 AND (u.is_admin OR EXISTS(SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=i.project_id AND m.enabled AND m.role='manager')))`, s.Namespace, s.ProjectID, p.Key.IntentID, s.ActorID, p.Key.Generation, p.Experiment.Hash, p.Experiment.PolicyRevision, p.Environment.ID, p.Environment.Hash).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return evolution.ErrForbidden
	}
	return nil
}

func evolutionPersistPlan(ctx context.Context, scope evolution.Scope, p evolution.ExecutionPlan) error {
	if _, err := evolutionActualMemoryHash(p.Memories); err != nil {
		return err
	}
	if err := evolutionValidatePlan(p); err != nil {
		return err
	}
	return evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := evolutionFence(ctx, tx, scope, p); err != nil {
			return err
		}
		data := evolutionJSON(evolutionEnvelope(p))
		hash := evolutionTextHash(string(data))
		_, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_case_plans(request_id,namespace,project_id,actor_id,plan,plan_hash) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, p.RequestID, scope.Namespace, scope.ProjectID, scope.ActorID, data, hash)
		if err != nil {
			return err
		}
		var prior string
		if err = tx.QueryRow(ctx, `SELECT plan_hash FROM pangu.evolution_case_plans WHERE request_id=$1 AND namespace=$2 AND project_id=$3 AND actor_id=$4 AND plan IS NOT NULL`, p.RequestID, scope.Namespace, scope.ProjectID, scope.ActorID).Scan(&prior); err != nil {
			return err
		}
		if prior != hash {
			return evolution.ErrConflict
		}
		return evolutionSubmissionTx(ctx, tx, scope, p)
	})
}

func evolutionSubmissionTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan) error {
	snap := evolutionSubmissionSnapshot(s, p)
	data := evolutionJSON(snap)
	hash := evolutionTextHash(string(data))
	_, err := tx.Exec(ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,created_by,input_hash) VALUES($1,$2,$3,'evolution',$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, p.RequestID, s.Namespace, s.ProjectID, evolutionTextHash("evolution:"+p.RequestID), hash, data, s.ActorID, evolutionTextHash(snap.Workflow.Input))
	if err != nil {
		return err
	}
	var prior string
	if err = tx.QueryRow(ctx, `SELECT payload_hash FROM pangu.submissions WHERE id=$1 AND namespace=$2 AND project_id=$3 AND operation='evolution' AND purged_at IS NULL`, p.RequestID, s.Namespace, s.ProjectID).Scan(&prior); err != nil {
		return err
	}
	if prior != hash {
		return evolution.ErrConflict
	}
	return nil
}

func evolutionReadPlanTx(ctx context.Context, tx pgx.Tx, namespace, request string) (evolution.Scope, evolution.ExecutionPlan, error) {
	s := evolution.Scope{Namespace: namespace}
	var raw []byte
	var hash string
	var envelope evolutionStoredPlan
	err := tx.QueryRow(ctx, `SELECT project_id::text,actor_id::text,plan,plan_hash FROM pangu.evolution_case_plans WHERE request_id=$1 AND namespace=$2 AND erased_at IS NULL`, request, namespace).Scan(&s.ProjectID, &s.ActorID, &raw, &hash)
	if err != nil {
		return s, envelope.Plan, err
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return s, envelope.Plan, err
	}
	if err = evolutionNormalizeMemories(envelope.Plan.Memories); err != nil {
		return s, envelope.Plan, err
	}
	if evolutionHash(envelope) != hash {
		return s, envelope.Plan, evolution.ErrConflict
	}
	envelope.Plan.Environment.Hash = envelope.EnvironmentHash
	return s, envelope.Plan, evolutionValidatePlan(envelope.Plan)
}
