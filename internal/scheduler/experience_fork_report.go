package scheduler

import (
	"context"
	"reflect"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// Checkpoints carry report references, not retained bodies. Re-read those bodies
// even on cache recovery, where workspace.copyReport is intentionally not called.
func checkForkReport(ctx context.Context, run string, out workflow.NodeOutput) error {
	if len(out.ReportIDs) == 0 {
		return nil
	}
	if len(out.ReportIDs) != 1 || len(out.ReportSHA256) != 64 {
		return experience.ErrInvalidated
	}
	actual, err := research.Checkpoint(ctx, run, out.RunID)
	if err != nil {
		return err
	}
	if len(actual.Reports) != 1 || actual.Reports[0].ID != out.ReportIDs[0] || workflow.ReportHash(actual.Reports[0].Markdown) != out.ReportSHA256 || !reflect.DeepEqual(actual.Lineage, out.Lineage) {
		return experience.ErrInvalidated
	}
	actualHash, err := workflow.OutputFingerprint(actual)
	if err != nil {
		return err
	}
	expectedHash, err := workflow.OutputFingerprint(out)
	if err != nil {
		return err
	}
	if actualHash != expectedHash {
		return experience.ErrInvalidated
	}
	return nil
}
