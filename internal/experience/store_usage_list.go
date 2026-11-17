package experience

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

type usageListRecord struct {
	cursor     string
	summary    UsageSummary
	references []Reference
}

func (s Store) ListUsages(ctx context.Context, scope Scope, versionID string, page Page) (UsagePage, error) {
	if !uuidValue(versionID) || page.Limit < 1 || page.Limit > 100 || page.AfterID != "" && !validHash(page.AfterID) {
		return UsagePage{}, ErrInvalid
	}
	return withStoreTx(ctx, s, scope, "usage_list", func(tx pgx.Tx, scope Scope) (UsagePage, error) {
		if err := s.verifyVersionMetadataTx(ctx, tx, scope, versionID); err != nil {
			return UsagePage{}, err
		}
		records, err := usagePageTx(ctx, tx, scope, versionID, page)
		if err != nil {
			return UsagePage{}, err
		}
		result := UsagePage{Items: []UsageSummary{}}
		for index, record := range records {
			if err = s.verifyUsageSourcesTx(ctx, tx, scope, record.references); err != nil {
				return UsagePage{}, err
			}
			if index < page.Limit {
				result.Items = append(result.Items, record.summary)
			}
		}
		if len(records) > page.Limit {
			result.NextCursor = records[page.Limit-1].cursor
		}
		return result, nil
	})
}

func usagePageTx(ctx context.Context, tx pgx.Tx, scope Scope, versionID string, page Page) ([]usageListRecord, error) {
	rows, err := tx.Query(ctx, `SELECT identity_hash,usage->'input'->>'run_id',usage->'input'->>'node_id',
	 (usage->'input'->>'round')::integer,(usage->'input'->>'generation')::bigint,
	 (usage->'input'->>'approval_revision')::bigint,(usage->>'created_at')::timestamptz,usage->'references'
	 FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2
	 AND usage->'references' @> jsonb_build_array(jsonb_build_object('version_id',$3::text))
	 AND identity_hash>$4 ORDER BY identity_hash LIMIT $5`, scope.Namespace, scope.ProjectID, versionID, page.AfterID, page.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []usageListRecord{}
	for rows.Next() {
		var record usageListRecord
		var refs []byte
		item := &record.summary
		if err = rows.Scan(&record.cursor, &item.RunID, &item.NodeID, &item.Iteration, &item.ApprovalGeneration, &item.ApprovalRevision, &item.CreatedAt, &refs); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(refs, &record.references); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s Store) verifyUsageSourcesTx(ctx context.Context, tx pgx.Tx, scope Scope, refs []Reference) error {
	if len(refs) == 0 || len(refs) > 4 {
		return ErrInvalidated
	}
	for _, ref := range refs {
		if !uuidValue(ref.VersionID) || !validHash(ref.BodyHash) || ref.DeliveryRevision < 1 {
			return ErrInvalidated
		}
		var bodyHash string
		if err := tx.QueryRow(ctx, `SELECT body_hash FROM pangu.experience_versions WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, ref.VersionID).Scan(&bodyHash); err != nil {
			return storageError(err)
		}
		if bodyHash != ref.BodyHash {
			return ErrInvalidated
		}
		if err := s.verifyVersionMetadataTx(ctx, tx, scope, ref.VersionID); err != nil {
			return err
		}
	}
	return nil
}
