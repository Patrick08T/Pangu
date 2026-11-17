package evidence

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestBundleReadsCurrentSourcesAndProtectsProjectBoundary(t *testing.T) {
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	project, run, node, report, request := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,'evidence','证据测试','unused','main','test')`, []any{project}},
		{`INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test','unused',true)`, []any{run, project}},
		{`INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,snapshot,payload_hash,run_id,task_id) VALUES($1::uuid,'evidence',$2,'submit',$1::text,$3,'hash',$4,$1::uuid)`, []any{request, project, `{"workflow":{"version_id":"frozen-version","nodes":[]}}`, run}},
		{`INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence) VALUES($1,$2,$3,'市场原文',$4,'{}','[]')`, []any{report, run, node, []byte("可独立核对的市场证据")}},
	}
	for _, s := range statements {
		if _, err = conn.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatal(err)
		}
	}
	bundle, err := Load(ctx, run, project)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(bundle)
	if !strings.Contains(string(raw), "frozen-version") || !strings.Contains(string(raw), "可独立核对") {
		t.Fatal("缺少已有平台原文")
	}
	if _, err = Load(ctx, run, uuid.NewString()); err == nil {
		t.Fatal("跨项目证据泄露")
	}
	if _, err = conn.Exec(ctx, `UPDATE pangu.research_reports SET body=$2 WHERE id=$1`, report, []byte("更新后的原始证据")); err != nil {
		t.Fatal(err)
	}
	bundle, err = Load(ctx, run, project)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(bundle)
	if !strings.Contains(string(raw), "更新后的原始证据") {
		t.Fatal("读取了缓存证据")
	}
	if _, err = conn.Exec(ctx, `UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1`, run); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(ctx, run, project); err == nil {
		t.Fatal("恢复了过期证据")
	}
}
