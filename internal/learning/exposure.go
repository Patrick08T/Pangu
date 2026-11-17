package learning

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func recordExposure(ctx context.Context, tx pgx.Tx, scope experience.Scope, source experience.Source) error {
	_, err := tx.Exec(ctx, `INSERT INTO pangu.learning_source_exposures
	 (consent_id,namespace,project_id,source_id,run_id,task_id,input_hash,unresolved)
	 SELECT c.id,c.namespace,c.project_id,c.source_id,c.run_id,s.task_id,
	 encode(sha256(convert_to(s.snapshot->'workflow'->>'input','UTF8')),'hex'),false
	 FROM pangu.learning_source_consents c JOIN pangu.submissions s
	 ON s.namespace=c.namespace AND s.project_id=c.project_id AND s.run_id=c.run_id
	 WHERE c.namespace=$1 AND c.project_id=$2 AND c.source_id=$3 AND c.source_kind=$4 AND c.source_hash=$5
	 AND nullif(s.snapshot->'workflow'->>'input','') IS NOT NULL ON CONFLICT(consent_id) DO NOTHING`,
		scope.Namespace, scope.ProjectID, source.ID, source.Kind, source.Hash)
	if err != nil {
		return err
	}
	return requireExposure(ctx, tx, scope, source)
}

func requireExposure(ctx context.Context, tx pgx.Tx, scope experience.Scope, source experience.Source) error {
	var found bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.learning_source_exposures e
	 JOIN pangu.learning_source_consents c ON c.id=e.consent_id WHERE e.namespace=$1 AND e.project_id=$2
	 AND e.source_id=$3 AND e.run_id=$4 AND c.source_kind=$5 AND c.source_hash=$6
	 AND NOT e.unresolved AND e.task_id IS NOT NULL AND e.input_hash IS NOT NULL)`, scope.Namespace, scope.ProjectID,
		source.ID, source.RunID, source.Kind, source.Hash).Scan(&found)
	if err == nil && !found {
		return experience.ErrForbidden
	}
	return err
}

func frozenInputHash(snapshot []byte) string {
	var value struct {
		Workflow struct {
			Input string `json:"input"`
		} `json:"workflow"`
	}
	if json.Unmarshal(snapshot, &value) != nil {
		return ""
	}
	return digest([]byte(value.Workflow.Input))
}
