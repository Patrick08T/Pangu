package api

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
)

func (h handler) versionSubmission(ctx context.Context, version, input, key string) (*submission.Record, error) {
	namespace := h.backend.Namespace()
	v, err := catalog.ReadVersion(ctx, namespace, version)
	if err != nil {
		return nil, err
	}
	if v.ProjectID == "" || v.Kind != "agent" && v.Kind != "workflow" {
		return nil, errors.Join(catalog.ErrInvalid, errors.New("仅可运行项目 Agent 或工作流版本"))
	}
	saved, err := submission.FindVersion(ctx, namespace, v.ProjectID, key, v.ID, input)
	if err == nil {
		return saved, nil
	}
	if !errors.Is(err, submission.ErrNotFound) {
		return nil, err
	}
	d, err := catalog.Compile(ctx, namespace, version, input)
	if err != nil {
		return nil, err
	}
	return h.backend.Submit(ctx, *d, key)
}
