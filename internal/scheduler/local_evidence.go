package scheduler

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/evidence"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// P4 本地执行的证据放在独立临时目录，不往用户工作区添加 .git 元数据。
func callWithEvidence(ctx context.Context, item execution.Attempt, input workflow.NodeInput, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	directory := input.Workspace
	if directory != "" && input.Revision != nil && input.Revision.LocalPerforce {
		var err error
		directory, err = os.MkdirTemp("", "pangu-local-evidence-")
		if err != nil {
			return workflow.NodeOutput{}, err
		}
		defer os.RemoveAll(directory)
		if err = os.Mkdir(filepath.Join(directory, ".git"), 0700); err != nil {
			return workflow.NodeOutput{}, err
		}
	}
	var localRef *workflow.EvidenceReference
	if directory != "" {
		bundle, err := evidence.Load(ctx, item.RunID, input.ProjectID)
		if err != nil {
			return workflow.NodeOutput{}, err
		}
		localRef, err = bundle.Materialize(directory)
		if err != nil {
			return workflow.NodeOutput{}, err
		}
		copy := *localRef
		if directory != input.Workspace {
			copy.ManifestPath = filepath.Join(directory, filepath.FromSlash(copy.ManifestPath))
		}
		input.PlatformEvidence = &copy
	}
	out, err := execute(ctx, input)
	if localRef != nil {
		err = errors.Join(err, evidence.Verify(directory, localRef))
	}
	out.RunID = item.NodeRunID
	if err == nil {
		err = workflow.ValidateOutput(input.Node, &out)
	}
	return out, err
}
