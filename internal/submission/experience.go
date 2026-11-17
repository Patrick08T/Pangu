package submission

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/jackc/pgx/v5"
)

// freezeAcceptedExperience is part of acceptance, not its idempotency payload.
// Purpose is supplied by trusted services, never copied from a public run request.
func freezeAcceptedExperience(ctx context.Context, tx pgx.Tx, namespace string, item *Record) error {
	purpose := item.Snapshot.Purpose
	if purpose == "" {
		purpose = "business"
	}
	if purpose != "business" && purpose != "learning" && purpose != "evaluation" {
		return experience.ErrInvalid
	}
	scope := experience.Scope{Namespace: namespace, ProjectID: item.ProjectID, ActorID: item.CreatedBy}
	if purpose != "business" || scope.ActorID == "" {
		_, err := (experience.Store{}).FreezeEmptyTx(ctx, tx, scope, item.ID, 1, purpose)
		return err
	}
	_, err := learning.NewStore().FreezeTx(ctx, tx, scope, item.ID, 1)
	return err
}
