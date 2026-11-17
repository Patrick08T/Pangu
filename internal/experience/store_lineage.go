package experience

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
	"unicode/utf8"
)

func (s Store) ReadRunLineage(ctx context.Context, scope Scope, query RunLineageQuery) (RunLineage, error) {
	if s.VerifyRun == nil {
		return RunLineage{}, ErrForbidden
	}
	if !validLineageQuery(query) {
		return RunLineage{}, ErrInvalid
	}
	return withStoreTx(ctx, s, scope, "lineage_read", func(tx pgx.Tx, scope Scope) (RunLineage, error) {
		binding, err := s.VerifyRun(ctx, tx, scope, query)
		if err != nil {
			return RunLineage{}, err
		}
		if !validRunBinding(scope, query, binding) {
			return RunLineage{}, ErrForbidden
		}
		result := RunLineage{ActualRunID: binding.ActualRunID}
		result.Watermark, err = s.auditWatermarkTx(ctx, tx, scope, binding)
		if err != nil {
			return RunLineage{}, err
		}
		result.Usages, err = s.auditUsagesTx(ctx, tx, scope, binding, query.UsagePage)
		if err != nil {
			return RunLineage{}, err
		}
		result.Artifacts, err = s.auditArtifactPageTx(ctx, tx, scope, binding, query.ArtifactPage)
		if err != nil {
			return RunLineage{}, err
		}
		result.ChainComplete = completeAuditChain(result, query)
		return result, nil
	})
}

func validLineageQuery(q RunLineageQuery) bool {
	return auditID(q.ActualRunID) && (q.RequestID == "" || auditID(q.RequestID)) && q.WatermarkGeneration >= 0 &&
		validAuditPage(q.UsagePage) && (q.UsagePage.AfterID == "" || validHash(q.UsagePage.AfterID)) && validAuditPage(q.ArtifactPage)
}

func auditID(id string) bool {
	return id != "" && len(id) <= 256 && utf8.ValidString(id) && !strings.ContainsAny(id, "\x00\r\n\t")
}

func validAuditPage(p Page) bool {
	return p.Limit >= 1 && p.Limit <= 100 && (p.AfterID == "" || auditID(p.AfterID))
}

func validRunBinding(scope Scope, q RunLineageQuery, b RunBinding) bool {
	return b.Namespace == scope.Namespace && b.ProjectID == scope.ProjectID && b.ActualRunID == q.ActualRunID &&
		auditID(b.RequestID) && b.WatermarkGeneration > 0 && (q.RequestID == "" || q.RequestID == b.RequestID) &&
		(q.WatermarkGeneration == 0 || q.WatermarkGeneration == b.WatermarkGeneration)
}

func (s Store) ReadArtifactSummary(ctx context.Context, scope Scope, id string) (ArtifactSummary, error) {
	if !auditID(id) {
		return ArtifactSummary{}, ErrInvalid
	}
	return withStoreTx(ctx, s, scope, "lineage_read", func(tx pgx.Tx, scope Scope) (ArtifactSummary, error) {
		return s.auditArtifactTx(ctx, tx, scope, id, map[string]bool{}, map[string]ArtifactSummary{})
	})
}

func completeAuditChain(result RunLineage, q RunLineageQuery) bool {
	if !result.Watermark.Present || !result.Watermark.Proven || result.Artifacts.Total == 0 || q.UsagePage.AfterID != "" || q.ArtifactPage.AfterID != "" || result.Usages.HasMore || result.Artifacts.HasMore {
		return false
	}
	present := map[string]bool{}
	for _, item := range result.Artifacts.Items {
		present[item.ID] = true
	}
	for _, item := range result.Artifacts.Items {
		if !item.Proven || item.Redacted {
			return false
		}
		for _, parent := range item.Parents {
			if !present[parent.ID] {
				return false
			}
		}
	}
	return !result.Watermark.Redacted
}
