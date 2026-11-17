package scheduler

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// Artifact-only scans cannot discover legacy persisted outputs without lineage.
// A consumed gate therefore also audits the run's published checkpoints/reports;
// absence of an artifact is a missing proof, not an empty dependency set.
func (r *nodeExperience) auditPublishedOutputs(ctx context.Context) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT c.output FROM pangu.workspace_checkpoints c
 JOIN pangu.workspace_runs w ON w.run_id=c.run_id WHERE c.run_id=$1 AND w.project_id=$2
 UNION ALL SELECT p.output FROM pangu.research_reports p
 JOIN pangu.workspace_runs w ON w.run_id=p.run_id WHERE p.run_id=$1 AND w.project_id=$2`, r.runID, r.scope.ProjectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return err
		}
		var output workflow.NodeOutput
		if json.Unmarshal(raw, &output) != nil {
			return experience.ErrInvalidated
		}
		if err = r.checkParents(ctx, workflow.NodeInput{Parents: map[string]workflow.NodeOutput{"published": output}}); err != nil {
			return err
		}
	}
	return rows.Err()
}
