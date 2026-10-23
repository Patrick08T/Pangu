package artifact

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestReportAtomicReplayAndArtifactExport(t *testing.T) {
	run := artifactRun(t)
	ctx := context.Background()
	node, id := uuid.NewString(), uuid.NewString()
	record, err := workspace.Get(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	output := workflow.NodeOutput{Text: "报告摘要", RunID: node, ReportIDs: []string{id}, Reports: []workflow.Report{{ID: id, Title: "中文报告", Markdown: "## 结论\n原始报告内容"}}}
	_, err = research.Persist(ctx, run, node, record.ProjectID, func() (workflow.NodeOutput, error) { return output, nil })
	if err != nil {
		t.Fatal(err)
	}
	out, err := research.Persist(ctx, run, node, record.ProjectID, func() (workflow.NodeOutput, error) {
		t.Fatal("已经保存报告却重新执行模型")
		return workflow.NodeOutput{}, nil
	})
	if err != nil || out.ReportIDs[0] != id {
		t.Fatalf("报告重放错误: %+v %v", out, err)
	}
	manifest, err := List(ctx, run)
	if err != nil || len(manifest.Files) != 1 || manifest.Files[0].ID != id {
		t.Fatalf("报告未进入清单: %+v %v", manifest, err)
	}
	_, body, _, err := Read(ctx, run, id, false)
	if err != nil || !bytes.Contains(body, []byte("原始报告内容")) {
		t.Fatalf("报告不可读取: %v", err)
	}
	if _, _, _, err := Read(ctx, uuid.NewString(), id, false); err == nil {
		t.Fatal("跨运行读取报告")
	}
	data, err := Export(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) != 2 {
		t.Fatalf("报告导出错误: %v", err)
	}
}
