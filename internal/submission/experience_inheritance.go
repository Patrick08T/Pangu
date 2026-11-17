package submission

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type InheritanceExperienceProof struct {
	Experiences    []workflow.Experience
	EmptyWatermark bool
	LegacyEmpty    bool
}

// ReadInheritanceExperienceTx binds the proof to a retained accepted submission.
// role must be resolved by the trusted caller from the actual catalog agent.
func ReadInheritanceExperienceTx(ctx context.Context, tx pgx.Tx, namespace, run, role string, input workflow.NodeInput) (InheritanceExperienceProof, error) {
	var proof InheritanceExperienceProof
	item, _, err := scan(tx.QueryRow(ctx, "SELECT "+columns+` FROM pangu.submissions WHERE namespace=$1 AND run_id=$2 AND purged_at IS NULL`, namespace, run))
	if err != nil {
		return proof, err
	}
	if err = inheritanceInput(item, input); err != nil {
		return proof, err
	}
	selection, err := ExperienceSelection(item.ID, input, role, item.Operation)
	if err != nil {
		return proof, err
	}
	scope := experience.Scope{Namespace: namespace, ProjectID: item.ProjectID, ActorID: item.CreatedBy}
	mark, usage, versions, err := learning.NewStore().ReadFrozenSelectionTx(ctx, tx, scope, selection)
	proof.EmptyWatermark = mark.Hash != "" && len(mark.Versions) == 0
	if errors.Is(err, experience.ErrNotFound) && proof.EmptyWatermark && len(input.Experiences) == 0 {
		proof.LegacyEmpty = true
		return proof, nil
	}
	if err != nil {
		return proof, err
	}
	proof.Experiences, err = ExperiencePayload(usage, versions)
	return proof, err
}

func inheritanceInput(item *Record, input workflow.NodeInput) error {
	snapshot := item.Snapshot
	if item.CreatedBy == "" || snapshot.CreatedBy != item.CreatedBy || item.ProjectID != input.ProjectID || snapshot.Workflow.ProjectID != input.ProjectID || snapshot.Workflow.Input != input.Input || (input.RequestID != "" && input.RequestID != item.ID) || (snapshot.Purpose != "" && snapshot.Purpose != "business") {
		return experience.ErrInvalidated
	}
	actual, err := json.Marshal(input.Node)
	if err != nil {
		return err
	}
	for _, node := range snapshot.Workflow.Nodes {
		if node.ID != input.Node.ID {
			continue
		}
		expected, err := json.Marshal(node)
		if err != nil {
			return err
		}
		if string(actual) == string(expected) {
			return nil
		}
	}
	return experience.ErrInvalidated
}
