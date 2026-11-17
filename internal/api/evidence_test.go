package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"strings"
	"testing"
)

func TestEvidenceRouteUsesAuthorizedExactManifestPaths(t *testing.T) {
	h, _, project, run, token := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)VALUES($1,$2,'base','head','test','unused',true)`, run, project); err != nil {
		t.Fatal(err)
	}
	path := "/runs/" + run + "/evidence"
	w := callAuthorized(h, "GET", path, "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "snapshot.json") || strings.Contains(w.Body.String(), `"body"`) {
		t.Fatalf("索引应可读且不重复正文：%d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", path+"?path="+run+"/snapshot.json", "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), project) {
		t.Fatalf("缺少原始冻结证据：%d", w.Code)
	}
	for _, suffix := range []string{"?path=../../outside", "?path=foreign/snapshot.json"} {
		if w = callAuthorized(h, "GET", path+suffix, "", token); w.Code != 404 {
			t.Fatal("可越过证据索引读取", w.Code)
		}
	}
	if _, err = conn.Exec(ctx, "DELETE FROM pangu.memberships WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w = callAuthorized(h, "GET", path, "", token); w.Code != 403 {
		t.Fatal("移除项目权限后仍可读证据", w.Code)
	}
}
