package experience

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type versionListRecord struct {
	summary VersionSummary
	sources []Source
}

func (s Store) ListVersions(ctx context.Context, scope Scope, page Page) (VersionPage, error) {
	if page.Limit < 1 || page.Limit > 100 || page.AfterID != "" && !uuidValue(page.AfterID) {
		return VersionPage{}, ErrInvalid
	}
	return withStoreTx(ctx, s, scope, "version_list", func(tx pgx.Tx, scope Scope) (VersionPage, error) {
		records, err := versionPageTx(ctx, tx, scope, page)
		if err != nil {
			return VersionPage{}, err
		}
		result := VersionPage{Items: []VersionSummary{}}
		for index, record := range records {
			if err = s.verifySourcesTx(ctx, tx, scope, Version{ProjectID: scope.ProjectID, Sources: record.sources}); err != nil {
				return VersionPage{}, err
			}
			if index < page.Limit {
				result.Items = append(result.Items, record.summary)
			}
		}
		if len(records) > page.Limit {
			result.NextCursor = result.Items[len(result.Items)-1].ID
		}
		return result, nil
	})
}

func versionPageTx(ctx context.Context, tx pgx.Tx, scope Scope, page Page) ([]versionListRecord, error) {
	rows, err := tx.Query(ctx, `SELECT v.id::text,v.entry_id,v.revision,s.validity,d.mode,d.revision,
	 v.expires_at,v.created_at,v.sources FROM pangu.experience_versions v
	 JOIN pangu.experience_states s ON s.version_id=v.id
	 JOIN LATERAL (SELECT mode,revision FROM pangu.experience_deliveries WHERE version_id=v.id ORDER BY revision DESC LIMIT 1) d ON true
	 WHERE v.namespace=$1 AND v.project_id=$2 AND ($3='' OR v.id>NULLIF($3,'')::uuid)
	 ORDER BY v.id LIMIT $4`, scope.Namespace, scope.ProjectID, page.AfterID, page.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []versionListRecord{}
	for rows.Next() {
		var record versionListRecord
		var sources []byte
		item := &record.summary
		if err = rows.Scan(&item.ID, &item.EntryID, &item.Revision, &item.Validity, &item.Delivery, &item.DeliveryRevision, &item.ExpiresAt, &item.CreatedAt, &sources); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(sources, &record.sources); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func uuidValue(value string) bool { _, err := uuid.Parse(value); return err == nil }

func (s Store) verifyVersionMetadataTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) error {
	var sources []Source
	if err := scanJSON(tx.QueryRow(ctx, `SELECT sources FROM pangu.experience_versions WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id), &sources); err != nil {
		return err
	}
	return s.verifySourcesTx(ctx, tx, scope, Version{ProjectID: scope.ProjectID, Sources: sources})
}
