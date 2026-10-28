package factory

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/catalog"
)

func bindModels(ctx context.Context, ns string, b *catalog.Bundle) error {
	for i := range b.Agents {
		node := b.Agents[i].Node
		if node == nil || node.Executor != "eino" {
			continue
		}
		v, err := catalog.ReadVersion(ctx, ns, node.ConnectionVersion)
		if err != nil {
			return err
		}
		if v.Kind != "connection" || v.Archived || v.Spec.Connection == nil {
			return invalid("Eino 只能使用工作空间未归档的连接版本")
		}
		model := v.Spec.Connection.Model
		if node.Model != "" && node.Model != model {
			return invalid("候选模型与固定连接版本不一致")
		}
		node.Model = model
	}
	return nil
}
