package api

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
)

// Empty purpose is not proof of business ownership: old retention removed it.
// Only a surviving, scoped legacy definition can classify such a run as public.
func (h handler) businessRun(ctx context.Context, run scheduler.RunSummary) (bool, error) {
	if run.Purpose != "" {
		return run.Purpose == "business", nil
	}
	if run.ID == "" || run.ProjectID == "" {
		return false, nil
	}
	record, err := submission.ForRun(ctx, h.backend.Namespace(), run.ID)
	if errors.Is(err, submission.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if record == nil || record.RunID != run.ID || record.ProjectID != run.ProjectID {
		return false, nil
	}
	snapshot := record.Snapshot
	return businessPurpose(snapshot.Purpose) && snapshot.Workflow.ProjectID == run.ProjectID && len(snapshot.Workflow.Nodes) > 0, nil
}
