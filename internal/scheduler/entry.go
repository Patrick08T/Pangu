package scheduler

import (
	"context"
	"errors"
	"strings"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type entryHooks struct {
	preflight func(context.Context, workflow.Definition) (*workflow.Preflight, error)
	dispatch  func(context.Context, *submission.Record) *submission.Record
}

// SubmitEntry is the server-selected business entrance; callers cannot choose a version or purpose.
func (b Backend) SubmitEntry(ctx context.Context, projectID, entryID, input, key string) (*submission.Record, error) {
	return submitEntry(ctx, b.Namespace(), projectID, entryID, input, key, entryHooks{b.Preflight, b.dispatch})
}

func submitEntry(ctx context.Context, namespace, projectID, entryID, input, key string, hooks entryHooks) (*submission.Record, error) {
	project, err := uuid.Parse(projectID)
	if err != nil {
		return nil, errors.New("提交必须指定已登记的 project_id")
	}
	if namespace == "" || !submission.PublicKey(key) {
		return nil, errors.New("提交作用域或幂等键无效")
	}
	if !submission.ValidKey(entryID) || strings.Contains(entryID, ":") {
		return nil, errors.New("workbench entry_id 无效")
	}
	if err = (workflow.Definition{Input: input}).ValidateInput(); err != nil {
		return nil, err
	}
	r := entryRequest{namespace: namespace, project: project.String(), entry: "workbench:" + entryID, input: input, key: key}
	item, binding, err := loadEntry(ctx, r)
	if err != nil {
		return nil, err
	}
	if item != nil {
		return hooks.dispatch(ctx, item), nil
	}
	d, err := catalog.CompileScoped(ctx, namespace, binding.version, catalog.VersionScope{ProjectID: r.project, AllowObserving: true}, input)
	if err != nil {
		return nil, err
	}
	if _, err = d.Layers(); err != nil {
		return nil, err
	}
	check, err := hooks.preflight(ctx, *d)
	if err != nil {
		return nil, err
	}
	if check == nil || !check.Ready {
		return nil, ErrNotReady
	}
	snapshot := submission.Snapshot{Workflow: *d, Purpose: "business", EntryKey: r.entry, EntryRevision: binding.revision}
	item, err = acceptEntry(ctx, r, binding, snapshot)
	if err != nil {
		return nil, err
	}
	return hooks.dispatch(ctx, item), nil
}
