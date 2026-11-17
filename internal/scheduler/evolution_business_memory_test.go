//go:build integration

package scheduler

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestEvolutionBusinessMemoryDistinguishesActualAcceptedSelections(t *testing.T) {
	f := evolutionBusinessMemoryFixture(t)
	first := evolutionBusinessFixture(t, f, f.request.Payload.FlowVersion, "JSON paired business", "accepted", true)
	empty, err := evolutionBusinessRead(t, f, first)
	if err != nil {
		t.Fatal(err)
	}
	memory := evolutionBusinessMemoryVersion(t, f)
	second := evolutionBusinessFixture(t, f, f.request.Payload.FlowVersion, "JSON paired business", "accepted", true)
	full, err := evolutionBusinessRead(t, f, second)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = entryQuery(t, `SELECT count(*) FROM pangu.experience_usage WHERE namespace=$1 AND usage#>>'{input,run_id}'=$2 AND usage->'references' @> $3::jsonb`, f.scope.Namespace, full.RequestID, evolutionJSON([]experience.Reference{{VersionID: memory.ID, BodyHash: memory.BodyHash, DeliveryRevision: 2}})).Scan(&count); err != nil || count == 0 {
		t.Fatal("real request selection missing", count, err)
	}
	if empty.MemoryHash == full.MemoryHash || evolutionComparable(empty, full) {
		t.Fatal("actual memory injection was silently compared as empty", empty.MemoryHash, full.MemoryHash)
	}
	third := evolutionBusinessFixture(t, f, f.request.Payload.FlowVersion, "JSON paired business", "accepted", true)
	same, err := evolutionBusinessRead(t, f, third)
	if err != nil || !evolutionComparable(full, same) {
		t.Fatal("identical actual memory became incomparable due to request identity", err)
	}
	if full.MemoryEvidenceHash == same.MemoryEvidenceHash || len(full.MemoryEvidenceHash) != 64 {
		t.Fatal("source proof lost actual request/artifact/attempt identity")
	}
	entryExec(t, `UPDATE pangu.learning_source_consents SET revoked_at=now() WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID)
	if _, err = evolutionBusinessRead(t, f, second); err == nil {
		t.Fatal("withdrawn source consent remained comparable")
	}
}

func TestEvolutionBusinessMemoryRejectsMissingOrCorruptProof(t *testing.T) {
	cases := map[string]string{
		"extra unbound usage":      `INSERT INTO pangu.experience_usage(namespace,project_id,identity_hash,usage) SELECT namespace,project_id,repeat('0',64),usage FROM pangu.experience_usage WHERE namespace=$1 AND usage#>>'{input,run_id}'=(SELECT id::text FROM pangu.submissions WHERE run_id=$2) LIMIT 1`,
		"history missing node":     `UPDATE pangu.run_history SET summary=jsonb_set(summary,'{nodes}',(summary->'nodes')-0) WHERE namespace=$1 AND run_id=$2`,
		"history unknown node":     `UPDATE pangu.run_history SET summary=jsonb_set(summary,'{nodes,0,node_id}','"not-in-definition"') WHERE namespace=$1 AND run_id=$2`,
		"history foreign artifact": `UPDATE pangu.run_history SET summary=jsonb_set(summary,'{nodes,0,id}','"00000000-0000-0000-0000-000000000001"') WHERE namespace=$1 AND run_id=$2`,
		"attempt missing":          `DELETE FROM pangu.node_attempts WHERE run_id=$2 AND EXISTS(SELECT 1 FROM pangu.submissions WHERE run_id=$2 AND namespace=$1)`,
		"attempt wrong node":       `UPDATE pangu.node_attempts SET node_id='foreign-node' WHERE run_id=$2 AND EXISTS(SELECT 1 FROM pangu.submissions WHERE run_id=$2 AND namespace=$1)`,
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			f := evolutionBusinessMemoryFixture(t)
			run := evolutionBusinessFixture(t, f, f.request.Payload.FlowVersion, "JSON complete proof", "accepted", true)
			if _, err := evolutionBusinessRead(t, f, run); err != nil {
				t.Fatal("intact proof rejected", err)
			}
			entryExec(t, sql, f.scope.Namespace, run)
			if _, err := evolutionBusinessRead(t, f, run); err == nil {
				t.Fatal("missing or corrupt memory proof counted as known empty", name)
			}
		})
	}
}
