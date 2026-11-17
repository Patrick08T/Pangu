package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestLocalPerforceEvidenceStaysOutsideBoundDirectory(t *testing.T) {
	run, project := revisionRun(t)
	for _, tamper := range []bool{false, true} {
		t.Run(strconv.FormatBool(tamper), func(t *testing.T) {
			directory := t.TempDir()
			var manifest string
			input := workflow.NodeInput{RunID: run, ProjectID: project, Workspace: directory, Revision: &workflow.Revision{LocalPerforce: true}}
			_, err := callWithEvidence(context.Background(), execution.Attempt{RunID: run, NodeRunID: uuid.NewString()}, input, func(_ context.Context, in workflow.NodeInput) (workflow.NodeOutput, error) {
				if in.PlatformEvidence == nil {
					t.Fatal("没有提供平台证据")
				}
				manifest = in.PlatformEvidence.ManifestPath
				if !filepath.IsAbs(manifest) {
					t.Fatal("外部证据必须使用可读取的绝对路径")
				}
				if _, err := os.ReadFile(manifest); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(directory, ".git")); !os.IsNotExist(err) {
					t.Fatal("污染了绑定工作区")
				}
				if tamper {
					return workflow.NodeOutput{}, os.WriteFile(manifest, []byte("篡改"), 0600)
				}
				return workflow.NodeOutput{Text: "核对完成"}, nil
			})
			if (err != nil) != tamper {
				t.Fatalf("证据完整性验证不正确: %v", err)
			}
			if _, err := os.Stat(manifest); !os.IsNotExist(err) {
				t.Fatal("临时证据未清理")
			}
		})
	}
}
