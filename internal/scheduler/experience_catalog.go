package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
)

// Only persisted, already-authorized requests reach this boundary. Observing versions
// remain usable by their accepted entry, but frozen bodies never override revocation.
func checkAcceptedCatalog(ctx context.Context, namespace string, item *submission.Record) error {
	ids := []string{item.Snapshot.Workflow.VersionID}
	for _, node := range item.Snapshot.Workflow.Nodes {
		ids = append(ids, node.AgentVersion)
	}
	scope := catalog.VersionScope{ProjectID: item.ProjectID, AllowObserving: true}
	seen := map[string]bool{}
	for len(ids) > 0 {
		id := ids[0]
		ids = ids[1:]
		if id == "" || seen[id] {
			continue
		}
		version, err := catalog.ReadVersionScoped(ctx, namespace, id, scope)
		if err != nil {
			return err
		}
		if version.Archived {
			return catalog.ErrArchived
		}
		seen[id] = true
		ids = append(ids, version.PublicBaseline, version.ProjectPrevious)
		if version.Spec.Flow != nil {
			for _, node := range version.Spec.Flow.Nodes {
				ids = append(ids, node.AgentVersion)
			}
		}
	}
	return nil
}
