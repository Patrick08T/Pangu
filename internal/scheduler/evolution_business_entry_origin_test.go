//go:build integration

package scheduler

import "testing"

func TestEvolutionBusinessEntryRejectsForeignOriginsAndIncompleteProvenance(t *testing.T) {
	f := evolutionBusinessMemoryFixture(t)
	entryExec(t, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,'workbench:main',$3,$3)`, f.scope.Namespace, f.scope.ProjectID, f.request.Payload.FlowVersion)
	run := evolutionBusinessEntryRun(t, f, "accepted")
	original, err := evolutionBusinessRead(t, f, run)
	if err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]string{
		"purpose": `"trial"`, "approval_origin": `"foreign-approval"`, "source_run_id": `"00000000-0000-0000-0000-000000000001"`, "fork": `{}`, "entry_revision": `0`, "entry_key": `"cron:main"`, "created_by": `"00000000-0000-0000-0000-000000000001"`,
	} {
		t.Run(field, func(t *testing.T) {
			entryExec(t, `UPDATE pangu.submissions SET snapshot=jsonb_set($2::jsonb,ARRAY[$3::text],$4::jsonb) WHERE run_id=$1`, run, original.SnapshotBody, field, value)
			if _, err := evolutionBusinessRead(t, f, run); err == nil {
				t.Fatal("foreign origin counted as actual business entry", field)
			}
		})
	}
	entryExec(t, `UPDATE pangu.submissions SET snapshot=$2 WHERE run_id=$1`, run, original.SnapshotBody)
	if _, err = evolutionBusinessRead(t, f, run); err != nil {
		t.Fatal("original entry did not remain valid", err)
	}
}
