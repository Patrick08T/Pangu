package experience

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s Store) Read(ctx context.Context, scope Scope, id string) (Version, error) {
	return withStoreTx(ctx, s, scope, "version_read", func(tx pgx.Tx, scope Scope) (Version, error) {
		return s.readVersionTx(ctx, tx, scope, id)
	})
}

func (s Store) readVersionTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Version, error) {
	var v Version
	var sources, content, delivery []byte
	err := tx.QueryRow(ctx, `SELECT v.id::text,v.entry_id::text,v.revision,v.project_id::text,v.body_hash,v.sources,v.expires_at,
	 s.validity,s.tombstoned,s.invalid_reason,d.mode,d.revision,d.policy,b.content
	 FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id
	 JOIN LATERAL (SELECT * FROM pangu.experience_deliveries WHERE version_id=v.id ORDER BY revision DESC LIMIT 1) d ON true
	 LEFT JOIN pangu.experience_bodies b ON b.version_id=v.id WHERE v.namespace=$1 AND v.project_id=$2 AND v.id=$3`,
		scope.Namespace, scope.ProjectID, id).Scan(&v.ID, &v.EntryID, &v.Revision, &v.ProjectID, &v.BodyHash, &sources, &v.ExpiresAt,
		&v.Validity, &v.Tombstoned, &v.InvalidReason, &v.Delivery, &v.DeliveryRevision, &delivery, &content)
	if err != nil {
		return Version{}, storageError(err)
	}
	if v.Tombstoned || v.InvalidReason != "" || v.Validity == "revoked" || v.Validity == "expired" || !v.ExpiresAt.After(time.Now()) || len(content) == 0 {
		return Version{}, ErrInvalidated
	}
	if err = json.Unmarshal(sources, &v.Sources); err != nil {
		return Version{}, err
	}
	if err = s.verifySourcesTx(ctx, tx, scope, v); err != nil {
		return Version{}, err
	}
	if err = json.Unmarshal(content, &v.Content); err != nil {
		return Version{}, err
	}
	if err = json.Unmarshal(delivery, &v.Canary); err != nil {
		return Version{}, err
	}
	if err = v.Validate(); err != nil {
		return Version{}, err
	}
	if bodyHash(v.Content) != v.BodyHash {
		return Version{}, ErrInvalidated
	}
	return v, nil
}

func (s Store) Activate(ctx context.Context, scope Scope, id string, receipt Activation) (Version, error) {
	if s.VerifyClaim == nil {
		return Version{}, ErrForbidden
	}
	return withStoreTx(ctx, s, scope, "version_activate", func(tx pgx.Tx, scope Scope) (Version, error) {
		v, err := s.readVersionTx(ctx, tx, scope, id)
		if err != nil {
			return Version{}, err
		}
		if v.Validity != "candidate" {
			return Version{}, ErrConflict
		}
		intent, err := readIntentTx(ctx, tx, scope, receipt.IntentID)
		if err != nil {
			return Version{}, err
		}
		if err = s.authorizedIntentTx(ctx, tx, scope, intent.Input); err != nil {
			return Version{}, err
		}
		if intent.Generation != receipt.Generation {
			return Version{}, ErrForbidden
		}
		if err = s.VerifyClaim(ctx, tx, scope, v, receipt); err != nil {
			return Version{}, err
		}
		if err = activateVersionTx(ctx, tx, scope, v); err != nil {
			return Version{}, err
		}
		if err = auditTx(ctx, tx, scope, id, receipt); err != nil {
			return Version{}, err
		}
		v.Validity = "active"
		return v, nil
	})
}

func activateVersionTx(ctx context.Context, tx pgx.Tx, scope Scope, v Version) error {
	var newer bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id
	 WHERE v.namespace=$1 AND v.project_id=$2 AND v.entry_id=$3 AND v.revision>=$4 AND s.validity='active')`, scope.Namespace, scope.ProjectID, v.EntryID, v.Revision).Scan(&newer)
	if err != nil {
		return err
	}
	if newer {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.experience_states s SET validity='superseded',updated_at=now() FROM pangu.experience_versions v
	 WHERE s.version_id=v.id AND v.namespace=$1 AND v.project_id=$2 AND v.entry_id=$3 AND s.validity='active'`, scope.Namespace, scope.ProjectID, v.EntryID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.experience_states SET validity='active',updated_at=now() WHERE version_id=$1`, v.ID)
	return err
}
