//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func TestExperienceErasureProofAcceptedForkRequiresAllLineageHashes(t *testing.T) {
	for _, mode := range []string{"valid", "legacy", "wrong-source", "wrong-input"} {
		t.Run(mode, func(t *testing.T) {
			f := newErasureFixture(t, "ttl")
			ancestor := uuid.NewString()
			request := uuid.NewString()
			f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id) VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,'{}',$4)`, request, f.ns, f.project, ancestor)
			a := f.artifact(t, ancestor, uuid.NewString(), []experience.Reference{f.ref}, nil)
			lineage := map[string]any{"version": 1, "artifact_id": a.ID, "input_hash": a.InputHash, "output_hash": a.Hash, "execution_input_hash": a.ExecutionInputHash}
			if mode == "wrong-input" {
				lineage["input_hash"] = a.Hash
			}
			if mode == "legacy" {
				delete(lineage, "execution_input_hash")
			}
			source := ancestor
			if mode == "wrong-source" {
				source = uuid.NewString()
			}
			snapshot, _ := json.Marshal(map[string]any{"source_run_id": source, "fork": map[string]any{"reused": map[string]any{"answer": map[string]any{"lineage": lineage, "text": "do not select this private body"}}}})
			f.exec(t, `UPDATE pangu.submissions SET snapshot=$2 WHERE id=$1`, f.request, snapshot)
			tx, err := f.conn.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			roots, err := experienceErasureProofTx(context.Background(), tx, experienceErasureBinding{f.ns, f.project, f.request, f.run})
			if mode == "valid" {
				if err != nil || len(roots) != 1 {
					t.Fatalf("accepted fork was not discovered before artifact creation: %v %v", roots, err)
				}
			} else if err == nil || len(roots) != 0 {
				t.Fatalf("unproven fork granted authority: %v %v", roots, err)
			}
		})
	}
}
