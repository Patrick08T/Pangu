package artifact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestReadOnlyNodeCannotPublishUnexpectedFiles(t *testing.T) {
	run := artifactRun(t)
	ctx := context.Background()
	_, err := workspace.Execute(ctx, run, uuid.NewString(), 1, true, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "违规写入"}, os.WriteFile(filepath.Join(dir, "unexpected.txt"), []byte("changed"), 0600)
	})
	if err == nil {
		t.Fatal("只读节点发布了文件修改")
	}
	set, err := workspace.Files(ctx, run)
	if err != nil || len(set.Changes) != 0 {
		t.Fatalf("只读失败污染成功工作区: %v", err)
	}
	_, err = workspace.Execute(ctx, run, uuid.NewString(), 1, true, func(string) (workflow.NodeOutput, error) { return workflow.NodeOutput{Text: "只读完成"}, nil })
	if err != nil {
		t.Fatal(err)
	}
}
