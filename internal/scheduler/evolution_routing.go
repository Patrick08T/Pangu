package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
)

type submissionSender func(context.Context, *submission.Record) (string, error)

func routeSubmission(ctx context.Context, namespace string, item *submission.Record, private, business submissionSender) (string, error) {
	if item.Operation == "evolution" || item.Snapshot.Purpose == "evaluation" {
		if item.Operation != "evolution" || item.Snapshot.Purpose != "evaluation" {
			return "", evolution.ErrForbidden
		}
		return private(ctx, item)
	}
	if err := checkAcceptedCatalog(ctx, namespace, item); err != nil {
		return "", err
	}
	return business(ctx, item)
}
