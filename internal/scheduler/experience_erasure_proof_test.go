//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func (f erasureFixture) usage(t *testing.T, request string, valid bool) {
	t.Helper()
	refs := []experience.Reference{f.ref}
	mark := experience.Watermark{RunID: request, Generation: 1, PolicyRevision: 1, Versions: []experience.FrozenVersion{{Reference: f.ref, Delivery: "serving"}}}
	raw, _ := json.Marshal(mark)
	mark.Hash = invalidationDigest(raw)
	encoded, _ := json.Marshal(mark)
	f.exec(t, `INSERT INTO pangu.experience_watermarks(namespace,project_id,run_id,generation,snapshot) VALUES($1,$2,$3,1,$4)`, f.ns, f.project, request, encoded)
	input := experience.SelectionInput{ProjectID: f.project, RunID: request, NodeID: "answer", Round: 1, Generation: 1, WatermarkGeneration: 1, ApprovalRevision: 1, BoundInputHash: strings.Repeat("c", 64)}
	raw, _ = json.Marshal(refs)
	injection := invalidationDigest(raw)
	if !valid {
		injection = strings.Repeat("f", 64)
	}
	usage := experience.Usage{Input: input, References: refs, WatermarkHash: mark.Hash, InjectionHash: injection}
	encoded, _ = json.Marshal(usage)
	f.exec(t, `INSERT INTO pangu.experience_usage(namespace,project_id,identity_hash,usage) VALUES($1,$2,$3,$4)`, f.ns, f.project, invalidationDigest([]byte(request)), encoded)
}

func (f erasureFixture) artifact(t *testing.T, run, id string, refs []experience.Reference, parents []experience.ArtifactReference) experience.Artifact {
	t.Helper()
	a := experience.Artifact{ID: id, ProjectID: f.project, RunID: run, NodeID: "answer", Kind: "node_output", Hash: strings.Repeat("d", 64), InputHash: strings.Repeat("c", 64), ExecutionInputHash: strings.Repeat("e", 64), ProofVersion: 1, Generation: 1, References: refs, Parents: parents}
	raw, _ := json.Marshal(a)
	f.exec(t, `INSERT INTO pangu.experience_artifacts(namespace,project_id,id,artifact) VALUES($1,$2,$3,$4)`, f.ns, f.project, id, raw)
	return a
}

func TestExperienceErasureProofUsesRequestNotActualRunForUsage(t *testing.T) {
	for _, mode := range []string{"request", "actual-alias", "bad-injection"} {
		t.Run(mode, func(t *testing.T) {
			f := newErasureFixture(t, "ttl")
			request := f.request
			if mode == "actual-alias" {
				request = f.run
			}
			f.usage(t, request, mode != "bad-injection")
			tx, err := f.conn.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			roots, err := experienceErasureProofTx(context.Background(), tx, experienceErasureBinding{namespace: f.ns, project: f.project, request: f.request, run: f.run})
			if mode == "request" {
				if err != nil || len(roots) != 1 || roots[0] != f.ref {
					t.Fatalf("accepted usage lost causal authority: %+v %v", roots, err)
				}
			} else if len(roots) != 0 {
				t.Fatal("alias or invalid injection authorized erasure")
			}
		})
	}
}

func TestExperienceErasureProofFollowsExactAncestorHash(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-hash", "missing", "cycle"} {
		t.Run(mode, func(t *testing.T) {
			f := newErasureFixture(t, "withdrawn")
			ancestorRun := uuid.NewString()
			request := uuid.NewString()
			f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id) VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,'{}',$4)`, request, f.ns, f.project, ancestorRun)
			id := uuid.NewString()
			parent := experience.ArtifactReference{ID: id, Hash: strings.Repeat("d", 64)}
			if mode != "missing" {
				f.artifact(t, ancestorRun, id, []experience.Reference{f.ref}, nil)
			}
			if mode == "wrong-hash" {
				parent.Hash = strings.Repeat("f", 64)
			}
			child := uuid.NewString()
			if mode == "cycle" {
				parent.ID = child
			}
			f.artifact(t, f.run, child, nil, []experience.ArtifactReference{parent})
			tx, err := f.conn.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			roots, err := experienceErasureProofTx(context.Background(), tx, experienceErasureBinding{namespace: f.ns, project: f.project, request: f.request, run: f.run})
			if mode == "valid" {
				if err != nil || len(roots) != 1 {
					t.Fatalf("transitive cause lost: %+v %v", roots, err)
				}
			} else if err == nil || len(roots) > 0 {
				t.Fatal("unproven ancestor authorized erasure")
			}
		})
	}
}
