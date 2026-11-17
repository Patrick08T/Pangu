package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type experienceErasureGraph struct {
	tx        pgx.Tx
	binding   experienceErasureBinding
	artifacts map[string]experience.Artifact
	visiting  map[string]bool
	roots     map[string]experience.Reference
	checked   map[string]bool
}

func experienceErasureDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (g *experienceErasureGraph) references(ctx context.Context, refs []experience.Reference) error {
	if len(refs) > 4 {
		return ErrExperienceErasureDeferred
	}
	for _, ref := range refs {
		key := fmt.Sprintf("%s:%s:%d", ref.VersionID, ref.BodyHash, ref.DeliveryRevision)
		if g.checked[key] {
			continue
		}
		due, err := experienceErasureRootTx(ctx, g.tx, g.binding.namespace, g.binding.project, ref)
		if err != nil {
			return err
		}
		g.checked[key] = true
		if due {
			g.roots[key] = ref
		}
	}
	return nil
}

func (g *experienceErasureGraph) visit(ctx context.Context, ref experience.ArtifactReference, depth int) (experience.Artifact, error) {
	if ref.ID == "" || len(ref.ID) > 256 || !experienceErasureHash(ref.Hash) || depth > 64 || g.visiting[ref.ID] {
		return experience.Artifact{}, ErrExperienceErasureDeferred
	}
	if a, ok := g.artifacts[ref.ID]; ok {
		if a.Hash != ref.Hash {
			return a, ErrExperienceErasureDeferred
		}
		return a, nil
	}
	if len(g.artifacts)+len(g.visiting) >= 1024 {
		return experience.Artifact{}, ErrExperienceErasureDeferred
	}
	a, err := g.load(ctx, ref)
	if err != nil {
		return a, err
	}
	g.visiting[ref.ID] = true
	defer delete(g.visiting, ref.ID)
	if err = g.references(ctx, a.References); err != nil {
		return a, err
	}
	for _, parent := range a.Parents {
		if _, err = g.visit(ctx, parent, depth+1); err != nil {
			return a, err
		}
	}
	g.artifacts[ref.ID] = a
	return a, nil
}

func (g *experienceErasureGraph) load(ctx context.Context, ref experience.ArtifactReference) (experience.Artifact, error) {
	var a experience.Artifact
	// Immutable metadata only; never materialize arbitrary invalid_reason text.
	err := g.tx.QueryRow(ctx, `SELECT artifact-'invalid_reason' FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND id=$3`, g.binding.namespace, g.binding.project, ref.ID).Scan(&a)
	if err == pgx.ErrNoRows {
		err = ErrExperienceErasureDeferred
	}
	if err != nil {
		return a, err
	}
	if a.ID != ref.ID || a.Hash != ref.Hash || a.ProjectID != g.binding.project || a.ProofVersion != 1 || a.Generation < 1 || !experienceErasureHash(a.InputHash) || !experienceErasureHash(a.ExecutionInputHash) || len(a.Parents) > 1024 {
		return a, ErrExperienceErasureDeferred
	}
	if _, err = uuid.Parse(a.RunID); err != nil {
		return a, ErrExperienceErasureDeferred
	}
	var bound bool
	err = g.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.submissions s JOIN pangu.projects p ON p.id=s.project_id AND p.namespace=s.namespace WHERE s.namespace=$1 AND s.project_id=$2 AND s.run_id=$3 AND coalesce(s.snapshot->>'purpose','') IN ('','business'))`, g.binding.namespace, g.binding.project, a.RunID).Scan(&bound)
	if err == nil && !bound {
		err = ErrExperienceErasureDeferred
	}
	return a, err
}
