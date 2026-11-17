package evaluation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

// Persist real retained submission rows without importing submission, whose
// learning-benefit integration legitimately imports this package. Task lineage
// is still assigned by the actual database trigger, not manufactured in tests.
func databaseSubmissionFixture(t *testing.T, namespace string, definition workflow.Definition, sourceRun string, dispatched bool) (id, task, run string) {
	t.Helper()
	snapshot := struct {
		Workflow    workflow.Definition `json:"workflow"`
		SourceRunID string              `json:"source_run_id,omitempty"`
	}{definition, sourceRun}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	id = uuid.NewString()
	if dispatched {
		run = uuid.NewString()
	}
	err = conn.QueryRow(ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,input_hash,run_id)
 VALUES($1,$2,$3,'evaluation-test',$4,$5,$6,$7,nullif($8,'')::uuid) RETURNING task_id::text`,
		id, namespace, definition.ProjectID, textHash(id), textHash(string(payload)), payload, textHash(definition.Input), run).Scan(&task)
	if err != nil {
		t.Fatal(err)
	}
	return id, task, run
}
