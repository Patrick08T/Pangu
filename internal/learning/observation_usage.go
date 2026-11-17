package learning

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func observationUsage(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, policy experience.Policy, runID string, run businessRun) (string, error) {
	var request string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND run_id=$3`, scope.Namespace, scope.ProjectID, runID).Scan(&request); err != nil {
		return "", err
	}
	rows, err := tx.Query(ctx, `SELECT usage FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND usage->'input'->>'run_id'=$3 ORDER BY identity_hash`, scope.Namespace, scope.ProjectID, request)
	if err != nil {
		return "", err
	}
	blobs, err := pgx.CollectRows(rows, pgx.RowTo[[]byte])
	if err != nil {
		return "", err
	}
	usages := []experience.Usage{}
	for _, data := range blobs {
		var usage experience.Usage
		if err = json.Unmarshal(data, &usage); err != nil {
			return "", err
		}
		if usage.Input.ProjectID != scope.ProjectID || usage.Input.RunID != request || usage.InjectionHash != jsonHash(usage.References) {
			return "", experience.ErrInvalidated
		}
		if err = observationReferences(usage.References, version); err != nil {
			return "", err
		}
		if len(usage.References) > 0 {
			if err = observationWatermark(ctx, tx, scope, policy, version, usage); err != nil {
				return "", err
			}
			if err = observationTime(ctx, tx, version, policy, usage.CreatedAt, run.FinishedAt); err != nil {
				return "", err
			}
		}
		usages = append(usages, usage)
	}
	if err = observationArtifacts(ctx, tx, scope, version, runID, run.Summary, usages); err != nil {
		return "", err
	}
	return jsonHash(usages), nil
}

func observationWatermark(ctx context.Context, tx pgx.Tx, scope experience.Scope, policy experience.Policy, version experience.Version, usage experience.Usage) error {
	var data []byte
	generation := usage.Input.WatermarkGeneration
	if generation == 0 {
		generation = usage.Input.Generation
	}
	err := tx.QueryRow(ctx, `SELECT snapshot FROM pangu.experience_watermarks WHERE namespace=$1 AND project_id=$2 AND run_id=$3 AND generation=$4`, scope.Namespace, scope.ProjectID, usage.Input.RunID, generation).Scan(&data)
	if err != nil {
		return err
	}
	var mark experience.Watermark
	if err = json.Unmarshal(data, &mark); err != nil {
		return err
	}
	hash := mark.Hash
	mark.Hash = ""
	if mark.RunID != usage.Input.RunID || mark.Generation != generation || mark.PolicyRevision != policy.Revision || jsonHash(mark) != hash || hash != usage.WatermarkHash {
		return experience.ErrInvalidated
	}
	for _, frozen := range mark.Versions {
		if frozen.Reference.VersionID == version.ID && frozen.Delivery == version.Delivery && observationReferences([]experience.Reference{frozen.Reference}, version) == nil {
			return nil
		}
	}
	return experience.ErrInvalidated
}

func observationTime(ctx context.Context, tx pgx.Tx, version experience.Version, policy experience.Policy, started, finished time.Time) error {
	var deliveryStart, now time.Time
	err := tx.QueryRow(ctx, `SELECT created_at,clock_timestamp() FROM pangu.experience_deliveries WHERE version_id=$1 AND revision=$2`, version.ID, version.DeliveryRevision).Scan(&deliveryStart, &now)
	if err != nil {
		return err
	}
	window := deliveryStart.Add(time.Duration(policy.Observation.WindowSeconds) * time.Second)
	deadline := deliveryStart.Add(time.Duration(policy.Observation.DeadlineSeconds) * time.Second)
	retained := finished.Add(time.Duration(policy.Observation.RetentionSeconds) * time.Second)
	if started.Before(deliveryStart) || started.After(window) || finished.Before(started) || finished.After(deadline) || finished.After(now) || now.After(retained) {
		return experience.ErrInvalidated
	}
	return nil
}
