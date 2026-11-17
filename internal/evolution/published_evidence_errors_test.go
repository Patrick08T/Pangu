package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPublishedEvidenceErrorClassifiesOnlyConfirmedInvalidity(t *testing.T) {
	for _, cause := range []error{evaluation.ErrConflict, evaluation.ErrNotFound, evaluation.ErrInvalid, ErrChecker, ErrForbidden} {
		got := publishedEvidenceError(cause)
		if !errors.Is(got, ErrPublishedEvidence) || !errors.Is(got, cause) {
			t.Fatal("lost ineligibility classification or original cause", cause, got)
		}
	}
}

func TestPublishedEvidenceErrorPreservesUnknownReadAndCancellation(t *testing.T) {
	sql := &pgconn.PgError{Code: "22012", Message: "division by zero"}
	for _, cause := range []error{nil, context.Canceled, context.DeadlineExceeded, sql, errors.New("unknown storage failure"), errors.Join(evaluation.ErrConflict, sql), errors.Join(ErrChecker, context.DeadlineExceeded)} {
		got := publishedEvidenceError(cause)
		if got != cause || errors.Is(got, ErrPublishedEvidence) {
			t.Fatal("unknown outcome was rewritten as known invalidity", cause, got)
		}
	}
}
