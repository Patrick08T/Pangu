package evolution

import (
	"context"
	"reflect"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/jackc/pgx/v5"
)

func (s Service) publicationGuard(scope Scope, expected *intentRecord, experimentHash string) catalog.PublishGuard {
	return func(ctx context.Context, tx pgx.Tx, identity catalog.ExperimentIdentity, request catalog.AutomaticPublishRequest) error {
		intent, p, err := authorizedIntentTx(ctx, tx, scope, expected.ID, expected.Generation)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if intent.State != "passed" || identity.Hash != experimentHash || identity.ID != intent.ExperimentID || identity.Generation != intent.Generation || identity.LogicalProjectID != scope.ProjectID || !reflect.DeepEqual(request, intent.Config.Publish) {
			return ErrConflict
		}
		if _, err = s.recheckMemoriesTx(ctx, tx, scope, intent.Config); err != nil {
			return err
		}
		env, err := environmentTx(ctx, tx, scope, intent.Config.Request.EnvironmentID)
		if err != nil {
			return err
		}
		if env.Hash != identity.EnvironmentHash {
			return ErrConflict
		}
		entries, err := s.reservationEntriesTx(ctx, tx, scope, intent.Config.Request)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(entries, intent.Config.Entries) {
			return ErrConflict
		}
		return nil
	}
}
