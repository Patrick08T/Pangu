package catalog

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

// CompileDraft 固定已保存修订，试运行不创建发布版本。
func CompileDraft(ctx context.Context, namespace, id string, revision int, project, input string) (*workflow.Definition, error) {
	if _, err := uuid.Parse(project); err != nil {
		return nil, invalid("试运行必须指定有效项目")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	item, err := lockedObject(ctx, tx, namespace, id, revision)
	if err != nil {
		return nil, err
	}
	if item.Kind != "agent" {
		return nil, invalid("仅支持 Agent 草稿试运行")
	}
	if err = item.NewObject.Validate(); err != nil {
		return nil, invalid(err.Error())
	}
	if err = validateReferences(ctx, tx, namespace, item.NewObject, true); err != nil {
		return nil, err
	}
	out := &workflow.Definition{ProjectID: project, Input: input, Nodes: []workflow.Node{*item.Spec.Agent}}
	if err = out.ValidateInput(); err != nil {
		return nil, invalid(err.Error())
	}
	if _, err = out.Layers(); err != nil {
		return nil, invalid(err.Error())
	}
	return out, nil
}
