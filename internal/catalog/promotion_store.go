package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func guardPromotion(ctx context.Context, tx pgx.Tx, namespace string, r AutomaticPublishRequest, guard PublishGuard) (*ExperimentIdentity, error) {
	snapshot, err := promotionIdentity(ctx, tx, namespace, r.ExperimentID, "")
	if err != nil {
		return nil, err
	}
	if err = guard(ctx, tx, *snapshot, r); err != nil {
		return nil, err
	}
	locked, err := promotionIdentity(ctx, tx, namespace, r.ExperimentID, " FOR UPDATE")
	if err != nil {
		return nil, err
	}
	if snapshot.Hash != locked.Hash || snapshot.Generation != locked.Generation || snapshot.DecisionHash != locked.DecisionHash {
		return nil, ErrConflict
	}
	return locked, nil
}

func promotionIdentity(ctx context.Context, reader rowReader, namespace, id, lock string) (*ExperimentIdentity, error) {
	e, err := readExperimentLock(ctx, reader, namespace, id, lock)
	if err != nil {
		return nil, err
	}
	var outcome, hash string
	err = reader.QueryRow(ctx, `SELECT outcome,experiment_hash,decision_hash FROM pangu.evaluation_experiment_decisions
 WHERE experiment_id=$1 AND generation=$2 ORDER BY id DESC LIMIT 1`, id, e.Generation).Scan(&outcome, &hash, &e.DecisionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if outcome != "passed" || hash != e.Hash || e.DecisionHash == "" {
		return nil, ErrConflict
	}
	return e, nil
}

func lockPublishObjects(ctx context.Context, tx pgx.Tx, namespace string, r AutomaticPublishRequest) (map[string]Object, error) {
	targets := []PublishTarget{r.Agent}
	if r.Flow != nil {
		if r.Flow.ObjectID == r.Agent.ObjectID {
			return nil, ErrConflict
		}
		targets = append(targets, *r.Flow)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ObjectID < targets[j].ObjectID })
	out := map[string]Object{}
	for _, target := range targets {
		item, err := lockedObject(ctx, tx, namespace, target.ObjectID, target.ExpectedRevision)
		if err != nil {
			return nil, err
		}
		out[item.ID] = *item
	}
	return out, nil
}

func existingPublication(ctx context.Context, tx pgx.Tx, namespace string, e ExperimentIdentity, r AutomaticPublishRequest) (*AutomaticPublication, error) {
	agent, err := existingAutoVersion(ctx, tx, namespace, e, r.Agent)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &AutomaticPublication{Agent: *agent}
	if r.Flow != nil {
		out.Flow, err = existingAutoVersion(ctx, tx, namespace, e, *r.Flow)
		if err != nil {
			return nil, ErrConflict
		}
	}
	return out, nil
}

func existingAutoVersion(ctx context.Context, reader rowReader, namespace string, e ExperimentIdentity, target PublishTarget) (*Version, error) {
	v, err := scanVersion(reader.QueryRow(ctx, `SELECT `+versionColumns+` FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 WHERE o.namespace=$1 AND v.object_id=$2 AND v.source_experiment=$3 AND v.project_id=$4`, namespace, target.ObjectID, e.ID, e.LogicalProjectID))
	if err != nil {
		return nil, err
	}
	if v.PublicBaseline != target.PublicBaseline || v.ProjectPrevious != target.ProjectPrevious || v.Tombstoned || v.State == "revoked" {
		return nil, ErrConflict
	}
	return v, nil
}

func publishTarget(ctx context.Context, tx pgx.Tx, namespace string, e ExperimentIdentity, item Object, target PublishTarget, baseline string, spec Spec) (*Version, error) {
	base, err := readVersionScoped(ctx, tx, namespace, baseline, VersionScope{ProjectID: e.LogicalProjectID})
	if err != nil {
		return nil, err
	}
	public, err := latestTargetVersion(ctx, tx, namespace, item.ID, "")
	if err != nil {
		return nil, err
	}
	current, err := latestTargetVersion(ctx, tx, namespace, item.ID, e.LogicalProjectID)
	if err != nil {
		return nil, err
	}
	if err = validatePublishTarget(item, *base, *public, *current, target); err != nil {
		return nil, err
	}
	if (item.Kind == "agent" && spec.Agent == nil) || (item.Kind == "workflow" && spec.Flow == nil) {
		return nil, ErrConflict
	}
	return insertAutoVersion(ctx, tx, namespace, e, target, spec)
}

func latestTargetVersion(ctx context.Context, reader rowReader, namespace, object, project string) (*Version, error) {
	return scanVersion(reader.QueryRow(ctx, `SELECT `+versionColumns+` FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 WHERE o.namespace=$1 AND v.object_id=$2 AND `+scopedVersionFilter+` ORDER BY v.number DESC LIMIT 1`, namespace, object, nullableID(project), false))
}

func insertAutoVersion(ctx context.Context, tx pgx.Tx, namespace string, e ExperimentIdentity, target PublishTarget, spec Spec) (*Version, error) {
	id := uuid.NewString()
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec,source_kind,source_experiment,project_id,public_baseline,project_previous,content_hash)
 SELECT $1,$2,coalesce(max(number),0)+1,NULL,'{}'::jsonb,'auto',$3,$4,$5,$6,$7 FROM pangu.catalog_versions WHERE object_id=$2`,
		id, target.ObjectID, e.ID, e.LogicalProjectID, target.PublicBaseline, nullableID(target.ProjectPrevious), contentHash(spec))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.catalog_version_bodies(version_id,spec) VALUES($1,$2)`, id, data)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.catalog_version_state(version_id,state) VALUES($1,'observing')`, id)
	if err != nil {
		return nil, err
	}
	return readVersionScoped(ctx, tx, namespace, id, VersionScope{ProjectID: e.LogicalProjectID, AllowObserving: true})
}
