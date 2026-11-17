//go:build integration

package scheduler

import "testing"

func TestEvolutionBusinessCoverageRejectsPartialRoleSnapshots(t *testing.T) {
	for name, nodes := range map[string]string{
		"empty_nodes":         `[]`,
		"empty_node":          `[{}]`,
		"missing_executor":    `[{"id":"agent","instruction":"answer"}]`,
		"missing_instruction": `[{"id":"agent","executor":"eino"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			f, id, request := evolutionCoverageFixture(t)
			entryExec(t, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, id, evolutionJSON(map[string]any{"agent_id": request.Snapshot.Workflow.Nodes[0].AgentVersion}))
			entryExec(t, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{workflow,nodes}',$2::jsonb) WHERE id=$1`, request.ID, nodes)
			complete, err := evolutionCoverageRead(t, f, id)
			if err != nil || complete {
				t.Fatal("partial role-only candidate silently disappeared", complete, err)
			}
		})
	}
}

func TestEvolutionBusinessCoverageAllowsCompleteNoncandidateInlineNodes(t *testing.T) {
	f, id, request := evolutionCoverageFixture(t)
	entryExec(t, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, id, evolutionJSON(map[string]any{"agent_id": request.Snapshot.Workflow.Nodes[0].AgentVersion}))
	nodes := request.Snapshot.Workflow.Nodes
	for i := range nodes {
		nodes[i].AgentVersion = ""
	}
	// AgentVersion is optional for valid inline nodes, unlike ID/executor/instruction.
	entryExec(t, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{workflow,nodes}',$2::jsonb) WHERE id=$1`, request.ID, evolutionJSON(nodes))
	complete, err := evolutionCoverageRead(t, f, id)
	if err != nil || !complete {
		t.Fatal("normal inline nodes became unknown candidate solely for absent agent_version", complete, err)
	}
}
