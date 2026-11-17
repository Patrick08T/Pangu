package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func experienceRetentionDue(ctx context.Context, tx pgx.Tx, namespace, project, request string) (bool, error) {
	var frozen, current []byte
	var created, now time.Time
	var suite, caseID string
	err := tx.QueryRow(ctx, `SELECT p.created_at,clock_timestamp(),f.policy,c.policy,x.suite_id::text,e.case_id::text
	 FROM pangu.evolution_case_plans p JOIN pangu.evolution_executions e USING(request_id)
	 JOIN pangu.evolution_intents i ON i.id=e.intent_id AND i.namespace=p.namespace AND i.project_id=p.project_id
	 JOIN pangu.evaluation_experiments x ON x.id=i.experiment_id AND x.namespace=p.namespace AND x.project_id=p.project_id
	 JOIN pangu.experience_policies f ON f.namespace=p.namespace AND f.project_id=p.project_id AND f.revision=i.policy_revision
	 JOIN LATERAL(SELECT policy FROM pangu.experience_policies WHERE namespace=p.namespace AND project_id=p.project_id ORDER BY revision DESC LIMIT 1)c ON true
	 WHERE p.namespace=$1 AND p.project_id=$2 AND p.request_id=$3`, namespace, project, request).Scan(&created, &now, &frozen, &current, &suite, &caseID)
	if err != nil {
		return false, err
	}
	ttl, err := experienceRetentionSeconds(frozen, current)
	if err != nil {
		return false, err
	}
	withdrawn, err := experienceRetentionSource(ctx, tx, namespace, project, suite, caseID)
	if err != nil {
		return false, err
	}
	return withdrawn || now.Sub(created).Seconds() >= float64(ttl), nil
}

// Only the frozen grant and a shorter explicit subsequent grant bound the TTL.
// A disabled/read-paused policy (including an empty disabled policy) is not deletion consent.
func experienceRetentionSeconds(frozen, current []byte) (int64, error) {
	var original, latest experience.Policy
	if err := json.Unmarshal(frozen, &original); err != nil {
		return 0, err
	}
	if err := json.Unmarshal(current, &latest); err != nil {
		return 0, err
	}
	o, n := original.Observation, latest.Observation
	if o.RetentionSeconds <= 0 || !o.SourceRetentionAuthorized {
		return 0, errors.New("experience retention lacks frozen authorization")
	}
	seconds := o.RetentionSeconds
	if n.SourceRetentionAuthorized && n.RetentionSeconds > 0 && n.RetentionSeconds < seconds {
		seconds = n.RetentionSeconds
	}
	return seconds, nil
}

func experienceRetentionSource(ctx context.Context, tx pgx.Tx, namespace, project, suite, caseID string) (bool, error) {
	var source, task, hash string
	err := tx.QueryRow(ctx, `SELECT c.source_run_id::text,c.task_id::text,c.input_hash FROM pangu.evaluation_suite_cases c
	 JOIN pangu.evaluation_suites s ON s.id=c.suite_id WHERE s.namespace=$1 AND s.project_id=$2 AND c.suite_id=$3 AND c.case_id=$4`, namespace, project, suite, caseID).Scan(&source, &task, &hash)
	if err != nil {
		return false, err
	}
	var withdrawn bool
	err = tx.QueryRow(ctx, `SELECT archived OR expires_at<=clock_timestamp() FROM pangu.evaluation_cases
	 WHERE namespace=$1 AND project_id=$2 AND id=$3 FOR SHARE`, namespace, project, caseID).Scan(&withdrawn)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil || withdrawn {
		return withdrawn, err
	}
	var actualTask, actualHash string
	err = tx.QueryRow(ctx, `SELECT s.task_id::text,s.input_hash,s.purged_at IS NOT NULL
	 OR EXISTS(SELECT 1 FROM pangu.retention_records r WHERE r.namespace=s.namespace AND r.project_id=s.project_id AND r.run_id=s.run_id AND r.status='purged')
	 OR EXISTS(SELECT 1 FROM pangu.run_debug d WHERE d.run_id=s.run_id AND d.expired)
	 FROM pangu.submissions s WHERE s.namespace=$1 AND s.project_id=$2 AND s.run_id=$3 FOR SHARE OF s`, namespace, project, source).Scan(&actualTask, &actualHash, &withdrawn)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err == nil && (actualTask != task || actualHash != hash) {
		return false, errors.New("experience retention source identity mismatch")
	}
	return withdrawn, err
}
