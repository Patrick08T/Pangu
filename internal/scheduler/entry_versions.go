package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

// Immutable content was compiled before preflight; lifecycle/visibility must still hold at acceptance.
func checkEntryVersions(ctx context.Context, tx pgx.Tx, r entryRequest, d workflow.Definition) error {
	ids := map[string]bool{d.VersionID: true}
	for _, node := range d.Nodes {
		if node.AgentVersion != "" && node.AgentVersion != d.VersionID {
			ids[node.AgentVersion] = false
		}
	}
	for id, root := range ids {
		if err := checkEntryVersion(ctx, tx, r, id, root); err != nil {
			return err
		}
	}
	return nil
}

func checkEntryVersion(ctx context.Context, tx pgx.Tx, r entryRequest, id string, root bool) error {
	var source, project string
	var archived bool
	err := tx.QueryRow(ctx, `SELECT v.source_kind,coalesce(v.project_id::text,''),o.archived FROM pangu.catalog_versions v
 JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE v.id=$1 AND o.namespace=$2 FOR SHARE OF o`, id, r.namespace).Scan(&source, &project, &archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ErrNotFound
	}
	if err != nil {
		return err
	}
	if root && archived {
		return catalog.ErrNotFound
	}
	if source == "manual" {
		return nil
	}
	if source != "auto" || project != r.project {
		return catalog.ErrNotFound
	}
	var state string
	var present bool
	err = tx.QueryRow(ctx, `SELECT s.state,b.spec IS NOT NULL FROM pangu.catalog_version_state s
 JOIN pangu.catalog_version_bodies b ON b.version_id=s.version_id WHERE s.version_id=$1 FOR SHARE OF s,b`, id).Scan(&state, &present)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !present || (state != "stable" && state != "observing") {
		return catalog.ErrNotFound
	}
	return nil
}
