package scheduler

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

type nodeExperience struct {
	namespace, runID string
	request          *submission.Record
	scope            experience.Scope
	store            experience.Store
}

func loadNodeExperience(ctx context.Context, namespace, request, run, project string) (*nodeExperience, error) {
	if request == "" || run == "" || project == "" {
		return nil, experience.ErrForbidden
	}
	item, err := submission.Get(ctx, namespace, request)
	if err != nil {
		return nil, err
	}
	if item.ProjectID != project || item.Snapshot.Workflow.ProjectID != project || (item.RunID != "" && item.RunID != run) {
		return nil, experience.ErrForbidden
	}
	if err = checkAcceptedCatalog(ctx, namespace, item); err != nil {
		return nil, err
	}
	r := &nodeExperience{namespace: namespace, runID: run, request: item, scope: experience.Scope{Namespace: namespace, ProjectID: project, ActorID: item.CreatedBy}, store: learning.NewStore()}
	if item.CreatedBy == "" {
		r.store = experience.NewFrozenEmptyStore(r.authorizeEmpty)
	}
	if _, err = r.store.ReadWatermark(ctx, r.scope, request, 1); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *nodeExperience) Prepare(ctx context.Context, input *workflow.NodeInput) error {
	if err := checkAcceptedCatalog(ctx, r.namespace, r.request); err != nil {
		return err
	}
	if err := r.checkDefinition(*input); err != nil {
		return err
	}
	if err := r.checkParents(ctx, *input); err != nil {
		return err
	}
	input.RequestID = r.request.ID
	bound, err := r.request.Snapshot.Workflow.BindInput(*input)
	if err != nil {
		return err
	}
	*input = bound
	role, err := r.roleID(ctx, input.Node.AgentVersion)
	if err != nil {
		return err
	}
	selection, err := experienceSelection(r.request.ID, *input, role, r.request.Operation)
	if err != nil {
		return err
	}
	usage, versions, err := r.store.Select(ctx, r.scope, selection)
	if err != nil {
		return err
	}
	input.Experiences, err = experiencePayload(usage, versions)
	return err
}

func (r *nodeExperience) checkDefinition(input workflow.NodeInput) error {
	if input.ProjectID != r.scope.ProjectID || input.Input != r.request.Snapshot.Workflow.Input || (input.RequestID != "" && input.RequestID != r.request.ID) {
		return experience.ErrInvalidated
	}
	actual, err := json.Marshal(input.Node)
	if err != nil {
		return err
	}
	for _, node := range r.request.Snapshot.Workflow.Nodes {
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

func (r *nodeExperience) roleID(ctx context.Context, versionID string) (string, error) {
	if versionID == "" {
		return "", nil
	}
	version, err := catalog.ReadVersionScoped(ctx, r.namespace, versionID, catalog.VersionScope{ProjectID: r.scope.ProjectID, AllowObserving: true})
	if err != nil {
		return "", err
	}
	if version.Kind != "agent" || version.Archived {
		return "", experience.ErrInvalidated
	}
	return version.ObjectID, nil
}
