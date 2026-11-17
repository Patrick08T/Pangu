package scheduler

import (
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func revisionOrigin(gate approval.Gate) (workflow.NodeInput, error) {
	var subject struct {
		Input workflow.NodeInput `json:"node_input"`
	}
	if json.Unmarshal(gate.Subject, &subject) != nil || gate.Review == nil || subject.Input.RequestID == "" || subject.Input.Node.ID != gate.NodeID {
		return workflow.NodeInput{}, experience.ErrInvalidated
	}
	generation := subject.Input.ApprovalGeneration
	if generation == 0 {
		generation = 1
	}
	if generation != gate.ApprovalGeneration {
		return workflow.NodeInput{}, experience.ErrInvalidated
	}
	subject.Input.ApprovalGeneration = generation
	return subject.Input, nil
}
