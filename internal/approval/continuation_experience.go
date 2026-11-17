package approval

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func inheritedExperiences(ctx context.Context, tx pgx.Tx, namespace, run string, gate *Gate, input workflow.NodeInput) ([]workflow.Experience, error) {
	var subject struct {
		Input workflow.NodeInput `json:"node_input"`
	}
	if err := json.Unmarshal(gate.Subject, &subject); err != nil {
		return nil, err
	}
	original := subject.Input
	if original.ProjectID != gate.ProjectID || original.Node.ID != gate.NodeID || normalizedGeneration(original.ApprovalGeneration) != gate.ApprovalGeneration || original.ApprovalRound != gate.Round {
		return nil, ErrConflict
	}
	parent, err := inheritanceProof(ctx, tx, namespace, gate.RunID, original)
	if err != nil {
		return nil, err
	}
	child, err := inheritanceProof(ctx, tx, namespace, run, input)
	if err != nil {
		return nil, err
	}
	if (parent.LegacyEmpty || child.LegacyEmpty) && (!parent.EmptyWatermark || !child.EmptyWatermark || len(original.Experiences) != 0 || len(input.Experiences) != 0) {
		return nil, ErrConflict
	}
	for _, candidate := range [][]workflow.Experience{parent.Experiences, child.Experiences, input.Experiences} {
		same, err := sameExperiencePayload(original.Experiences, candidate)
		if err != nil {
			return nil, err
		}
		if !same {
			return nil, ErrConflict
		}
	}
	return child.Experiences, nil
}

func inheritanceProof(ctx context.Context, tx pgx.Tx, namespace, run string, input workflow.NodeInput) (submission.InheritanceExperienceProof, error) {
	role := ""
	if input.Node.AgentVersion != "" {
		version, err := catalog.ReadVersionScoped(ctx, namespace, input.Node.AgentVersion, catalog.VersionScope{ProjectID: input.ProjectID, AllowObserving: true})
		if err != nil {
			return submission.InheritanceExperienceProof{}, err
		}
		if version.Kind != "agent" || version.Archived {
			return submission.InheritanceExperienceProof{}, ErrConflict
		}
		role = version.ObjectID
	}
	return submission.ReadInheritanceExperienceTx(ctx, tx, namespace, run, role, input)
}

func sameExperiencePayload(a, b []workflow.Experience) (bool, error) {
	canonical := func(value []workflow.Experience) ([]byte, error) {
		if len(value) == 0 {
			return []byte("[]"), nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var decoded any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err = decoder.Decode(&decoded); err != nil {
			return nil, err
		}
		return json.Marshal(decoded)
	}
	left, err := canonical(a)
	if err != nil {
		return false, err
	}
	right, err := canonical(b)
	return bytes.Equal(left, right), err
}
