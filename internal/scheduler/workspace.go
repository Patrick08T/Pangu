package scheduler

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func (b Backend) RegisterProject(ctx context.Context, project workspace.Project) (*workspace.Project, error) {
	return workspace.Register(ctx, b.client.NewWorkflow(WorkflowName).GetName(), project)
}

func (b Backend) Projects(ctx context.Context) ([]workspace.Project, error) {
	return workspace.Projects(ctx, b.client.NewWorkflow(WorkflowName).GetName())
}

func inspectProject(ctx context.Context, namespace string, input preflightInput, result workflow.Preflight) workflow.Preflight {
	id := input.Workflow.ProjectID
	if id == "" {
		return result
	}
	project, err := workspace.ProjectByID(ctx, namespace, id)
	status := "unavailable"
	if err == nil && project.WorkerID == workspace.WorkerID() && filepath.IsAbs(os.Getenv("PANGU_WORKSPACE_ROOT")) {
		status = projectStatus(ctx, project, input.SourceRunID)
	}
	result.Checks = append(result.Checks, workflow.Check{Component: "project", Status: status})
	result.Ready = result.Ready && status == "ready"
	return result
}

func projectStatus(ctx context.Context, project *workspace.Project, source string) string {
	if len(project.Verification) > 0 {
		info, err := os.Stat(project.Verification[0])
		if err != nil || !info.Mode().IsRegular() {
			return "unavailable"
		}
	}
	if source != "" {
		if _, err := workspace.RerunSource(ctx, source, project.ID); err == nil {
			return "ready"
		}
		return "unavailable"
	}
	commit, err := workspace.Resolve(ctx, project.Source, project.Branch)
	if err == nil && workspace.Inspect(ctx, project.Source, commit) == nil {
		return "ready"
	}
	return "unavailable"
}
