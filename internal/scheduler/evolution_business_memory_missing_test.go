//go:build integration

package scheduler

import "testing"

func TestEvolutionBusinessMemoryMissingProofNeverMintsKnownEmptyUsage(t *testing.T) {
	f := evolutionBusinessMemoryFixture(t)
	run := evolutionBusinessFixture(t, f, f.request.Payload.FlowVersion, "JSON unexecuted proof", "accepted")
	if _, err := evolutionBusinessRead(t, f, run); err == nil {
		t.Fatal("unexecuted terminal metadata became known empty memory")
	}
	var count int
	if err := entryQuery(t, `SELECT count(*) FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID).Scan(&count); err != nil || count != 0 {
		t.Fatal("read-only observation minted usage", count, err)
	}
}
