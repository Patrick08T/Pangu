package experience

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s Store) Create(ctx context.Context, scope Scope, version Version) (Version, error) {
	if s.VerifySource == nil {
		return Version{}, ErrForbidden
	}
	return withStoreTx(ctx, s, scope, "version_create", func(tx pgx.Tx, scope Scope) (Version, error) {
		if version.ID == "" {
			version.ID = uuid.NewString()
		}
		if version.EntryID == "" {
			version.EntryID = uuid.NewString()
		}
		if version.ProjectID != scope.ProjectID || !version.ExpiresAt.After(time.Now()) {
			return Version{}, ErrInvalid
		}
		if err := version.Validate(); err != nil {
			return Version{}, err
		}
		if err := s.verifySourcesTx(ctx, tx, scope, version); err != nil {
			return Version{}, err
		}
		version.BodyHash, version.Validity, version.Delivery = bodyHash(version.Content), "candidate", "shadow"
		version.DeliveryRevision, version.Canary, version.Tombstoned, version.InvalidReason = 1, DeliveryPolicy{Mode: "shadow"}, false, ""
		if err := insertVersionTx(ctx, tx, scope, version); err != nil {
			return Version{}, err
		}
		if err := auditTx(ctx, tx, scope, version.ID, Reference{VersionID: version.ID, BodyHash: version.BodyHash, DeliveryRevision: 1}); err != nil {
			return Version{}, err
		}
		return version, nil
	})
}

func insertVersionTx(ctx context.Context, tx pgx.Tx, scope Scope, v Version) error {
	sources, err := json.Marshal(v.Sources)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, scope.Namespace, scope.ProjectID, v.EntryID, v.Revision, v.BodyHash, sources, v.ExpiresAt, scope.ActorID)
	if err != nil {
		return err
	}
	content, err := json.Marshal(v.Content)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO pangu.experience_bodies(version_id,content) VALUES($1,$2)`, v.ID, content); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'candidate')`, v.ID); err != nil {
		return err
	}
	return insertDeliveryTx(ctx, tx, scope, v.ID, 1, DeliveryPolicy{Mode: "shadow"}, "")
}

func (s Store) verifySourcesTx(ctx context.Context, tx pgx.Tx, scope Scope, version Version) error {
	if s.VerifySource == nil || len(version.Sources) == 0 {
		return ErrForbidden
	}
	for _, source := range version.Sources {
		if source.ProjectID != scope.ProjectID || !validHash(source.Hash) {
			return ErrForbidden
		}
		if err := s.VerifySource(ctx, tx, scope, source); err != nil {
			return err
		}
	}
	return nil
}
