package scheduler

import (
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func evolutionSubmissionSnapshot(s evolution.Scope, p evolution.ExecutionPlan) submission.Snapshot {
	d := p.Definition
	calls := int(p.MaxCalls)
	if calls > 4096 {
		calls = 4096
	}
	if d.Budget != nil && d.Budget.ModelCalls < calls {
		calls = d.Budget.ModelCalls
	}
	d.Budget = &workflow.Budget{ModelCalls: calls}
	return submission.Snapshot{Purpose: "evaluation", CreatedBy: s.ActorID, Workflow: d}
}
