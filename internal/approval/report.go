package approval

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func reportInput(ctx context.Context, run string, input workflow.NodeInput) (workflow.NodeInput, error) {
	input.RunID, input.Revision = run, nil
	parents, err := research.ParentContext(ctx, input)
	input.Parents = parents
	return input, err
}
