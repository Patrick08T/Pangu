package evaluation

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// This fixture persists the consent projection consumed by evaluation. The
// learning package separately verifies the actual GrantSource authorization API.
func exposureFixture(t *testing.T, ns string, spec SuiteSpec, withdrawn, unknown bool) string {
	t.Helper()
	run := spec.Cases[0].SourceRunID
	if unknown {
		run, _ = sourceRunFixture(t, ns, spec.ProjectID, "erased prior learning source")
	}
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	id := persistExposureFixture(t, tx, ns, spec, run, withdrawn, unknown)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return id
}

func persistExposureFixture(t *testing.T, tx pgx.Tx, ns string, spec SuiteSpec, run string, withdrawn, unknown bool) string {
	t.Helper()
	ctx, id := context.Background(), uuid.NewString()
	_, err := tx.Exec(ctx, `INSERT INTO pangu.learning_source_consents(id,namespace,project_id,source_id,source_kind,run_id,node_id,source_hash,actor_id,expires_at)
 SELECT $1,$2,$3,$4,'report',$5,'checker',$6,actor_id,now()+interval '1 day' FROM pangu.evaluation_cases WHERE id=$7`, id, ns, spec.ProjectID, uuid.NewString(), run, spec.Cases[0].ExpectedHash, spec.Cases[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if unknown {
		if _, err = tx.Exec(ctx, `SELECT pangu.purge_run_payloads($1)`, run); err != nil {
			t.Fatal(err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.learning_source_exposures(consent_id,namespace,project_id,source_id,run_id,task_id,input_hash,unresolved)
 SELECT c.id,c.namespace,c.project_id,c.source_id,c.run_id,s.task_id,
 CASE WHEN nullif(s.snapshot->'workflow'->>'input','') IS NOT NULL THEN encode(sha256(convert_to(s.snapshot->'workflow'->>'input','UTF8')),'hex') END,
 s.task_id IS NULL OR nullif(s.snapshot->'workflow'->>'input','') IS NULL
 FROM pangu.learning_source_consents c LEFT JOIN pangu.submissions s ON s.namespace=c.namespace AND s.project_id=c.project_id AND s.run_id=c.run_id WHERE c.id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn {
		if _, err = tx.Exec(ctx, `UPDATE pangu.learning_source_consents SET revoked_at=now() WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}
