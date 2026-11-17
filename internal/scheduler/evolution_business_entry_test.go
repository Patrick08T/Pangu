//go:build integration

package scheduler

import "testing"

func TestEvolutionBusinessEntryAcceptsServerSelectedBusinessObservation(t *testing.T) {
	f := evolutionBusinessMemoryFixture(t)
	old := f.request.Payload.FlowVersion
	next := evolutionNextFlowFixture(t, f, "")
	entryExec(t, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,'workbench:main',$3,$3)`, f.scope.Namespace, f.scope.ProjectID, old)
	baseline := evolutionBusinessEntryRun(t, f, "needs_changes")
	entryExec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$3,revision=revision+1 WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID, next.ID)
	candidate := evolutionBusinessEntryRun(t, f, "accepted")
	got, err := evolutionBusinessObserve(t, f, candidate)
	if err != nil || got.BaselineRunID != baseline || got.BaselineScore != 0 || got.CandidateScore != 1 {
		t.Fatal("server workbench entry excluded from observation", got, err)
	}
}
