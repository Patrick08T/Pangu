package scheduler

import (
	"context"
	"errors"
	"strings"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

type RunEntry struct {
	Key        string `json:"key"`
	Revision   int64  `json:"revision"`
	WorkflowID string `json:"workflow_id"`
	Current    bool   `json:"current"`
}

func runEntry(ctx context.Context, namespace string, record *submission.Record) (*RunEntry, error) {
	if record == nil || record.Operation != "entry" {
		return nil, nil
	}
	snapshot := record.Snapshot
	if snapshot.Purpose != "business" || snapshot.EntryRevision < 1 || !strings.HasPrefix(snapshot.EntryKey, "workbench:") ||
		record.ProjectID == "" || record.ProjectID != snapshot.Workflow.ProjectID || snapshot.Workflow.VersionID == "" {
		return nil, nil
	}
	entry := &RunEntry{Key: snapshot.EntryKey, Revision: snapshot.EntryRevision}
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := identity.Lock(ctx, tx, namespace); err != nil {
			return err
		}
		if err := experience.Lock(ctx, tx, namespace, record.ProjectID); err != nil {
			return err
		}
		req := entryRequest{namespace: namespace, project: record.ProjectID, entry: entry.Key}
		binding, err := readEntryBinding(ctx, tx, req)
		if errors.Is(err, identity.ErrForbidden) {
			return nil
		}
		if err != nil {
			return err
		}
		if binding.version != snapshot.Workflow.VersionID || binding.revision < entry.Revision {
			return nil
		}
		version, err := catalog.ReadVersionScopedTx(ctx, tx, namespace, binding.version, catalog.VersionScope{ProjectID: req.project, AllowObserving: true})
		if errors.Is(err, catalog.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if version.Kind == "workflow" && !version.Archived {
			entry.WorkflowID, entry.Current = version.ObjectID, true
		}
		return nil
	})
	return entry, err
}
