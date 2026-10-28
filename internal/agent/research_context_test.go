package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func reportContextInput(t *testing.T) (workflow.NodeInput, string) {
	t.Helper()
	setupCodex(t)
	project, _, _ := codexConnectionInput(t, "")
	in := codexInput("")
	in.ProjectID, in.RunID = project.ProjectID, uuid.NewString()
	in.Node.DependsOn = []string{"research"}
	report, node := uuid.NewString(), uuid.NewString()
	in.Parents = map[string]workflow.NodeOutput{"research": {Text: "竞品研究；报告产物：" + report, ReportIDs: []string{report}, RunID: node}}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)
	 VALUES($1,$2,'baseline','baseline','test',$3,true)`, in.RunID, in.ProjectID, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence)
	 VALUES($1,$2,$3,'竞品研究',$4,'{}','[]')`, report, in.RunID, node, []byte("## 细分市场\n任务交接是待验证机会。\n来源：https://example.com/product"))
	if err != nil {
		t.Fatal(err)
	}
	return in, report
}

func TestCodexReceivesOriginalDirectParentReport(t *testing.T) {
	in, report := reportContextInput(t)
	original := in.Parents["research"].Text
	for _, body := range []string{"任务交接是待验证机会。", "更新后的原始证据"} {
		out, err := Execute(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		var got struct{ Input workflow.NodeInput }
		if json.Unmarshal([]byte(out.Text), &got) != nil || !strings.Contains(got.Input.Parents["research"].Text, body) {
			t.Fatal("下游只拿到报告标题，没有实际报告正文")
		}
		if in.Parents["research"].Text != original || got.Input.Parents["research"].ReportIDs[0] != report {
			t.Fatal("改写了上游原始结果或丢失报告身份")
		}
		conn, err := database.Open(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Exec(context.Background(), "UPDATE pangu.research_reports SET body=$2 WHERE id=$1", report, []byte("更新后的原始证据"))
		conn.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestCodexRejectsInvalidParentReportBeforeExecution(t *testing.T) {
	in, report := reportContextInput(t)
	cases := map[string]func(*workflow.NodeInput){
		"跨运行":   func(v *workflow.NodeInput) { v.RunID = uuid.NewString() },
		"跨项目":   func(v *workflow.NodeInput) { v.ProjectID = uuid.NewString() },
		"非直接上游": func(v *workflow.NodeInput) { v.Node.DependsOn = nil },
		"错误节点": func(v *workflow.NodeInput) {
			p := v.Parents["research"]
			p.RunID = uuid.NewString()
			v.Parents = map[string]workflow.NodeOutput{"research": p}
		},
		"缺失报告": func(v *workflow.NodeInput) {
			p := v.Parents["research"]
			p.ReportIDs = []string{uuid.NewString()}
			v.Parents = map[string]workflow.NodeOutput{"research": p}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			input := in
			change(&input)
			out, err := Execute(context.Background(), input)
			if err == nil || len(out.Evidence) != 0 {
				t.Fatal("无效报告仍传给执行器", err)
			}
		})
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for _, body := range []string{"api_key=private-value-123", strings.Repeat("x", 1024*1024)} {
		if _, err = conn.Exec(context.Background(), "UPDATE pangu.research_reports SET body=$2 WHERE id=$1", report, []byte(body)); err != nil {
			t.Fatal(err)
		}
		out, err := Execute(context.Background(), in)
		if err == nil || len(out.Evidence) != 0 {
			t.Fatal("敏感或超长上下文仍传给执行器", err)
		}
	}
}
