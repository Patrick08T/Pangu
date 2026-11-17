package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestPerforceLoopReviewUsesCumulativeSnapshotRange(t *testing.T) {
	_, p, run, path := localP4Run(t)
	ctx := context.Background()
	_, err := Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{}, os.WriteFile(filepath.Join(dir, "edit.txt"), []byte("累计修复补丁"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	node := uuid.NewString()
	_, err = Execute(ctx, run, node, 1, true, func(dir string) (workflow.NodeOutput, error) {
		input := workflow.NodeInput{RunID: run, ProjectID: p.ID, NodeRunID: node, Workspace: dir,
			Iteration: &workflow.Iteration{ID: "repair", Round: 2, Gate: true}}
		r, err := RevisionFor(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err = json.Unmarshal(body, &fields); err != nil {
			t.Fatal(err)
		}
		repository, _ := fields["review_repository"].(string)
		if repository == "" || samePath(repository, dir) {
			t.Fatal("质量门没有独立的只读快照审查目录")
		}
		diff, err := git(ctx, repository, "diff", "--no-ext-diff", "--no-textconv", r.OriginCommit, r.HeadCommit, "--")
		if err != nil || !strings.Contains(diff, "+累计修复补丁") {
			t.Fatalf("快照差异未覆盖累计补丁: %s %v", diff, err)
		}
		tree, err := ContentRevision(ctx, run, r.HeadCommit)
		want, treeErr := git(ctx, repository, "rev-parse", r.HeadCommit+"^{tree}")
		if err != nil || treeErr != nil || tree != want {
			t.Fatalf("审查快照与质量门摘要不一致: %s %s %v %v", tree, want, err, treeErr)
		}
		return workflow.NodeOutput{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
		t.Fatal("绑定的 Perforce 目录不应被写入 Git 元数据")
	}
}
