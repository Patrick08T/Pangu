package experience

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s Store) SaveArtifact(ctx context.Context, scope Scope, artifact Artifact) error {
	_, err := withStoreTx(ctx, s, scope, "artifact_write", func(tx pgx.Tx, scope Scope) (bool, error) {
		if artifact.ProjectID != scope.ProjectID {
			return false, ErrForbidden
		}
		lookup := func(id string) (Artifact, error) {
			if id == artifact.ID {
				return artifact, nil
			}
			return artifactTx(ctx, tx, scope, id)
		}
		if err := ValidateArtifact(ArtifactReference{ID: artifact.ID, Hash: artifact.Hash}, lookup, func(refs []Reference) error { _, err := s.resolveReferencesTx(ctx, tx, scope, refs); return err }); err != nil {
			return false, err
		}
		data, err := json.Marshal(artifact)
		if err != nil {
			return false, err
		}
		old, err := artifactTx(ctx, tx, scope, artifact.ID)
		if err == nil {
			previous, _ := json.Marshal(old)
			if string(previous) != string(data) {
				return false, ErrConflict
			}
			return true, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return false, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_artifacts(namespace,project_id,id,artifact) VALUES($1,$2,$3,$4)`, scope.Namespace, scope.ProjectID, artifact.ID, data)
		return err == nil, err
	})
	return err
}

func artifactTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Artifact, error) {
	var artifact Artifact
	err := scanJSON(tx.QueryRow(ctx, `SELECT artifact FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id), &artifact)
	if err != nil {
		return Artifact{}, err
	}
	if artifact.ProjectID != scope.ProjectID || artifact.ID != id {
		return Artifact{}, ErrInvalidated
	}
	return artifact, nil
}

func (s Store) checkArtifactTx(ctx context.Context, tx pgx.Tx, scope Scope, ref ArtifactReference) error {
	return ValidateArtifact(ref, func(id string) (Artifact, error) { return artifactTx(ctx, tx, scope, id) }, func(refs []Reference) error { _, err := s.resolveReferencesTx(ctx, tx, scope, refs); return err })
}

func (s Store) CheckArtifact(ctx context.Context, scope Scope, ref ArtifactReference) error {
	_, err := withStoreTx(ctx, s, scope, "artifact_read", func(tx pgx.Tx, scope Scope) (bool, error) { return true, s.checkArtifactTx(ctx, tx, scope, ref) })
	return err
}

func (s Store) ValidateReferences(ctx context.Context, scope Scope, refs []Reference) error {
	_, err := withStoreTx(ctx, s, scope, "reference_read", func(tx pgx.Tx, scope Scope) ([]Version, error) { return s.resolveReferencesTx(ctx, tx, scope, refs) })
	return err
}

func (s Store) resolveReferencesTx(ctx context.Context, tx pgx.Tx, scope Scope, refs []Reference) ([]Version, error) {
	if s.frozenEmptyOnly && len(refs) > 0 {
		return nil, ErrForbidden
	}
	if len(refs) > 4 {
		return nil, ErrInvalid
	}
	if len(refs) > 0 {
		policy, err := policyTx(ctx, tx, scope)
		if err != nil {
			return nil, err
		}
		if !policy.ReadEnabled || policy.Revoked {
			return nil, ErrForbidden
		}
	}
	versions, total := []Version{}, 0
	seen := map[string]bool{}
	for _, ref := range refs {
		if seen[ref.VersionID] || ref.DeliveryRevision < 1 {
			return nil, ErrInvalidated
		}
		seen[ref.VersionID] = true
		v, err := s.readVersionTx(ctx, tx, scope, ref.VersionID)
		if err != nil {
			return nil, err
		}
		if err = checkFrozenSafetyTx(ctx, tx, v.ID); err != nil {
			return nil, err
		}
		if v.BodyHash != ref.BodyHash || (v.Validity != "active" && v.Validity != "superseded") {
			return nil, ErrInvalidated
		}
		var delivery DeliveryPolicy
		err = scanJSON(tx.QueryRow(ctx, `SELECT policy FROM pangu.experience_deliveries WHERE version_id=$1 AND revision=$2`, ref.VersionID, ref.DeliveryRevision), &delivery)
		if err != nil {
			return nil, err
		}
		if delivery.Mode != "canary" && delivery.Mode != "serving" {
			return nil, ErrInvalidated
		}
		v.Delivery, v.DeliveryRevision, v.Canary = delivery.Mode, ref.DeliveryRevision, delivery
		content, _ := json.Marshal(v.Content)
		total += len(content)
		if total > 8192 {
			return nil, ErrInvalidated
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (s Store) ReadArtifact(ctx context.Context, scope Scope, id string) (Artifact, error) {
	return withStoreTx(ctx, s, scope, "artifact_read", func(tx pgx.Tx, scope Scope) (Artifact, error) {
		artifact, err := artifactTx(ctx, tx, scope, id)
		if err != nil {
			return Artifact{}, err
		}
		if err = s.checkArtifactTx(ctx, tx, scope, ArtifactReference{ID: id, Hash: artifact.Hash}); err != nil {
			return Artifact{}, err
		}
		return artifact, nil
	})
}

func (s Store) CheckArtifactInput(ctx context.Context, scope Scope, ref ArtifactReference, expectedInputHash string) error {
	_, err := withStoreTx(ctx, s, scope, "artifact_read", func(tx pgx.Tx, scope Scope) (bool, error) {
		artifact, err := artifactTx(ctx, tx, scope, ref.ID)
		if err != nil {
			return false, err
		}
		if expectedInputHash == "" || artifact.InputHash != expectedInputHash {
			return false, ErrInvalidated
		}
		return true, s.checkArtifactTx(ctx, tx, scope, ref)
	})
	return err
}
