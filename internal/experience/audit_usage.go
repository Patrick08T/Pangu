package experience

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

type runUsageRecord struct {
	cursor  string
	summary RunUsageSummary
}

func (s Store) auditUsagesTx(ctx context.Context, tx pgx.Tx, scope Scope, b RunBinding, page Page) (RunUsagePage, error) {
	result := RunUsagePage{Items: []RunUsageSummary{}}
	err := tx.QueryRow(ctx, `SELECT count(*) FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND usage->'input'->>'run_id'=ANY($3::text[])`, scope.Namespace, scope.ProjectID, []string{b.RequestID, b.ActualRunID}).Scan(&result.Total)
	if err != nil {
		return result, err
	}
	records, err := runUsageRecordsTx(ctx, tx, scope, b, page)
	if err != nil {
		return result, err
	}
	for i, record := range records {
		item := record.summary
		item.AuditState, err = s.auditReferencesTx(ctx, tx, scope, item.References)
		if err != nil {
			return RunUsagePage{}, err
		}
		if item.InjectionHash != referenceHash(item.References) {
			item.AuditState = mergeAudit(item.AuditState, unprovenAudit("injection_hash_mismatch"))
		}
		if item.MatchReason != "scoped_error_or_keyword_match" && item.MatchReason != "no_reliable_match" {
			item.MatchReason = "unrecorded"
		}
		if item.Redacted {
			item.InjectionHash = ""
			item.References = []Reference{}
		}
		if item.References == nil {
			item.References = []Reference{}
		}
		if i < page.Limit {
			result.Items = append(result.Items, item)
		}
	}
	result.HasMore = len(records) > page.Limit
	if result.HasMore {
		result.NextCursor = records[page.Limit-1].cursor
	}
	return result, nil
}

func runUsageRecordsTx(ctx context.Context, tx pgx.Tx, scope Scope, b RunBinding, page Page) ([]runUsageRecord, error) {
	rows, err := tx.Query(ctx, `SELECT identity_hash,jsonb_build_object(
 'node_id',usage->'input'->'node_id','iteration',usage->'input'->'round',
 'approval_generation',usage->'input'->'generation','approval_revision',usage->'input'->'approval_revision',
 'match_reason',usage->'match_reason','injection_hash',usage->'injection_hash',
 'references',usage->'references','created_at',usage->'created_at')
 FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2
 AND usage->'input'->>'run_id'=ANY($3::text[]) AND identity_hash>$4 ORDER BY identity_hash LIMIT $5`, scope.Namespace, scope.ProjectID, []string{b.RequestID, b.ActualRunID}, page.AfterID, page.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []runUsageRecord{}
	for rows.Next() {
		var record runUsageRecord
		var data []byte
		if err = rows.Scan(&record.cursor, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &record.summary); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
