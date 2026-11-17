package experience

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s Store) Freeze(ctx context.Context, scope Scope, runID string, generation int64) (Watermark, error) {
	return withStoreTx(ctx, s, scope, "watermark_freeze", func(tx pgx.Tx, scope Scope) (Watermark, error) {
		return s.FreezeTx(ctx, tx, scope, runID, generation)
	})
}

func (s Store) FreezeTx(ctx context.Context, tx pgx.Tx, scope Scope, runID string, generation int64) (Watermark, error) {
	scope.Action = "watermark_freeze"
	if err := s.authorizeTx(ctx, tx, scope); err != nil {
		return Watermark{}, err
	}
	if runID == "" || generation < 1 {
		return Watermark{}, ErrInvalid
	}
	mark, err := watermarkTx(ctx, tx, scope, runID, generation)
	if err == nil {
		return mark, s.checkWatermarkSourcesTx(ctx, tx, scope, mark)
	}
	if !errors.Is(err, ErrNotFound) {
		return Watermark{}, err
	}
	policy, err := policyTx(ctx, tx, scope)
	if err != nil {
		return Watermark{}, err
	}
	mark = Watermark{RunID: runID, Generation: generation, PolicyRevision: policy.Revision, Versions: []FrozenVersion{}}
	if policy.Enabled && policy.ReadEnabled && !policy.Revoked {
		mark.Versions, err = s.freezeVersionsTx(ctx, tx, scope)
		if err != nil {
			return Watermark{}, err
		}
	}
	mark.Hash = watermarkHash(mark)
	data, err := json.Marshal(mark)
	if err != nil {
		return Watermark{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_watermarks(namespace,project_id,run_id,generation,snapshot) VALUES($1,$2,$3,$4,$5)`, scope.Namespace, scope.ProjectID, runID, generation, data)
	if err != nil {
		return Watermark{}, storageError(err)
	}
	if err = auditTx(ctx, tx, scope, runID, map[string]any{"generation": generation, "hash": mark.Hash}); err != nil {
		return Watermark{}, err
	}
	return mark, s.Authorize(ctx, tx, scope)
}

func (s Store) freezeVersionsTx(ctx context.Context, tx pgx.Tx, scope Scope) ([]FrozenVersion, error) {
	rows, err := tx.Query(ctx, `SELECT v.id::text FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id
	 LEFT JOIN LATERAL (SELECT mode FROM pangu.experience_deliveries WHERE version_id=v.id ORDER BY revision DESC LIMIT 1) d ON true
	 WHERE v.namespace=$1 AND v.project_id=$2 AND s.validity='active' AND NOT s.tombstoned AND v.expires_at>now()
	 AND (d.mode IS NULL OR d.mode IN ('canary','serving'))
	 ORDER BY v.entry_id,v.id`, scope.Namespace, scope.ProjectID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	result := []FrozenVersion{}
	for _, id := range ids {
		v, err := s.readVersionTx(ctx, tx, scope, id)
		if err != nil {
			return nil, err
		}
		blocked, err := frozenSafetyBlockedTx(ctx, tx, v.ID)
		if err != nil {
			return nil, err
		}
		if blocked || v.Delivery != "canary" && v.Delivery != "serving" {
			continue
		}
		result = append(result, FrozenVersion{Reference: versionReference(v), Delivery: v.Delivery, Canary: v.Canary})
	}
	return result, nil
}

func watermarkHash(mark Watermark) string {
	mark.Hash = ""
	data, _ := json.Marshal(mark)
	return hashBytes(data)
}

func watermarkTx(ctx context.Context, tx pgx.Tx, scope Scope, runID string, generation int64) (Watermark, error) {
	var mark Watermark
	err := scanJSON(tx.QueryRow(ctx, `SELECT snapshot FROM pangu.experience_watermarks WHERE namespace=$1 AND project_id=$2 AND run_id=$3 AND generation=$4`, scope.Namespace, scope.ProjectID, runID, generation), &mark)
	if err != nil {
		return Watermark{}, err
	}
	if mark.RunID != runID || mark.Generation != generation || watermarkHash(mark) != mark.Hash {
		return Watermark{}, ErrInvalidated
	}
	return mark, nil
}

func (s Store) ReadWatermark(ctx context.Context, scope Scope, runID string, generation int64) (Watermark, error) {
	return withStoreTx(ctx, s, scope, "watermark_read", func(tx pgx.Tx, scope Scope) (Watermark, error) {
		mark, err := s.readWatermarkTx(ctx, tx, scope, runID, generation)
		if err != nil {
			return Watermark{}, err
		}
		return mark, s.checkWatermarkSourcesTx(ctx, tx, scope, mark)
	})
}

func (s Store) checkWatermarkSourcesTx(ctx context.Context, tx pgx.Tx, scope Scope, mark Watermark) error {
	for _, frozen := range mark.Versions {
		v, err := s.readVersionTx(ctx, tx, scope, frozen.Reference.VersionID)
		if err != nil {
			return err
		}
		if v.BodyHash != frozen.Reference.BodyHash {
			return ErrInvalidated
		}
	}
	return nil
}

func versionReference(v Version) Reference {
	return Reference{VersionID: v.ID, BodyHash: v.BodyHash, DeliveryRevision: v.DeliveryRevision}
}
