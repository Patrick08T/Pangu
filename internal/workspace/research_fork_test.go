package workspace

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestResearchForkPreservesMarkdownAndRejectsChangedSource(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	project, source, target, node, report := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err = db.Exec(ctx, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,'research-fork','研究返工','unused','main','test')`, project); err != nil {
		t.Fatal(err)
	}
	for _, run := range []string{source, target} {
		if _, err = db.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)VALUES($1,$2,'base','head','test','unused',true)`, run, project); err != nil {
			t.Fatal(err)
		}
	}
	body := "# 原始市场报告\n已核验资料。"
	out := workflow.NodeOutput{RunID: node, Text: "研究摘要", ReportIDs: []string{report}}
	raw, _ := json.Marshal(out)
	if _, err = db.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence)VALUES($1,$2,$3,'研究',$4,$5,'[]')`, report, source, node, []byte(body), raw); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"checkpoints": map[string]string{"research": "base"}, "reused": map[string]workflow.NodeOutput{"research": out}, "report_nodes": map[string]bool{"research": true}, "report_hashes": map[string]string{"research": workflow.ReportHash(body)}})
	var fork workflow.Fork
	if err = json.Unmarshal(input, &fork); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE pangu.research_reports SET body=$2 WHERE id=$1`, report, []byte("替换报告")); err != nil {
		t.Fatal(err)
	}
	if _, err = ImportCheckpoints(ctx, target, &fork); err == nil {
		t.Fatal("接受了变化的来源报告")
	}
	if _, err = db.Exec(ctx, `UPDATE pangu.research_reports SET body=$2 WHERE id=$1`, report, []byte(body)); err != nil {
		t.Fatal(err)
	}
	values, err := ImportCheckpoints(ctx, target, &fork)
	if err != nil {
		t.Fatal("原生研究报告不能复用", err)
	}
	copy := values["research"]
	if copy.ReportSHA256 != "" || copy.ReportIDs[0] == report {
		t.Fatal("改变了研究正文格式或没有新身份")
	}
	var saved []byte
	if err = db.QueryRow(ctx, "SELECT body FROM pangu.research_reports WHERE id=$1", copy.ReportIDs[0]).Scan(&saved); err != nil || string(saved) != body {
		t.Fatal("复制正文不同", err)
	}
}
