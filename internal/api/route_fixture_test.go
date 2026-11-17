package api

import (
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// calls is retained for older tests; route assertions distinguish reads from
// mutations so an added authority lookup is not mistaken for a submitted action.
func (b *fakeBackend) readCall() {
	b.calls++
	b.reads++
}

func (b *fakeBackend) mutationCall() {
	b.calls++
	b.mutations++
}

func routeRequest(id string, definition workflow.Definition) *submission.Record {
	if len(definition.Nodes) == 0 {
		if err := json.Unmarshal([]byte(validRun), &definition); err != nil {
			panic(err)
		}
	}
	return &submission.Record{ID: id, ProjectID: definition.ProjectID, RunID: id, Status: "accepted", Snapshot: submission.Snapshot{Purpose: "business", Workflow: definition}}
}
