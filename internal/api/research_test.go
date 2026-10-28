package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestReportEvidenceReviewBindsArtifactAndChecksPermissions(t *testing.T) {
	h, _, project, run, token := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test','unused',true)`, run, project)
	if err != nil {
		t.Fatal(err)
	}
	report := uuid.NewString()
	metadata := `{"mode":"planning","tasks":[],"claims":[{"id":"c1","statement":"需要试点","kind":"unknown","scope":"当前团队","uncertainty":"样本不足","conflicts":[],"evidence":[]}],"sources":[]}`
	_, err = conn.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,'规划',$4,'{}','[]',$5)`, report, run, uuid.NewString(), []byte("报告正文"), metadata)
	if err != nil {
		t.Fatal(err)
	}
	path := "/runs/" + run + "/reports"
	w := callAuthorized(h, "GET", path, "", token)
	var items []struct {
		ID          string `json:"id"`
		SubjectHash string `json:"subject_hash"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &items); err != nil || w.Code != 200 || len(items) != 1 || items[0].ID != report {
		t.Fatalf("报告详情: %d %s", w.Code, w.Body.String())
	}
	review := path + "/" + report + "/claims/c1"
	body := `{"revision":0,"subject_hash":"` + items[0].SubjectHash + `","verdict":"partial","reason":"范围正确，但样本不足"}`
	if w = callAuthorized(h, "PUT", review, body, token); w.Code != 403 {
		t.Fatalf("查看者可以核验: %d", w.Code)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	assertClaimReviewRejectsToken(t, h, project, review, body)
	if w = callAuthorized(h, "PUT", review, strings.ReplaceAll(body, items[0].SubjectHash, strings.Repeat("b", 64)), token); w.Code != 409 {
		t.Fatalf("错误报告摘要被接受: %d %s", w.Code, w.Body.String())
	}
	for range 2 {
		if w = callAuthorized(h, "PUT", review, body, token); w.Code != 200 {
			t.Fatalf("核验或重试失败: %d %s", w.Code, w.Body.String())
		}
	}
	assertClaimRevocationAfterWait(t, project, run, report, items[0].SubjectHash)
	if _, err = conn.Exec(ctx, "DELETE FROM pangu.research_reports WHERE id=$1", report); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = conn.QueryRow(ctx, "SELECT count(*) FROM pangu.claim_reviews WHERE report_id=$1", report).Scan(&count); err != nil || count != 0 {
		t.Fatalf("报告清理后遗留核验正文: %d %v", count, err)
	}
}
