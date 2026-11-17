package scheduler

import (
	"strings"

	"github.com/Patrick08T/Pangu/internal/submission"
)

func evolutionBusinessOrigin(run evolutionBusinessRun) bool {
	snap := run.Snapshot
	if snap.Fork != nil || snap.SourceRunID != "" || snap.ApprovalOrigin != "" || snap.CreatedBy != run.OwnerID {
		return false
	}
	if run.Operation != "entry" {
		return snap.Purpose == "" || snap.Purpose == "business"
	}
	if snap.Purpose != "business" || snap.EntryRevision < 1 || snap.EntryKey != run.EntryKey || run.OwnerID == "" || !strings.HasPrefix(snap.EntryKey, "workbench:") {
		return false
	}
	entry := strings.TrimPrefix(snap.EntryKey, "workbench:")
	return submission.ValidKey(entry) && !strings.Contains(entry, ":")
}
