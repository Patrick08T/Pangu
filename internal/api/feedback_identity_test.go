package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPITokenCannotImpersonateHumanFeedback(t *testing.T) {
	h, ns, project, run, session := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var user string
	if err = conn.QueryRow(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1 RETURNING user_id::text", project).Scan(&user); err != nil {
		t.Fatal(err)
	}
	_, token, err := identity.IssueToken(ctx, identity.Actor{ID: user, Namespace: ns}, project, "自动化", []string{"read", "manage_run"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	path := "/runs/" + run + "/feedback"
	var state struct {
		SubjectHash string `json:"subject_hash"`
	}
	w := callAuthorized(h, "GET", path, "", session)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &state) != nil {
		t.Fatal("缺少摘要")
	}
	body := `{"revision":0,"decision":"accepted","reason":"人已核对","subject_hash":"` + state.SubjectHash + `"}`
	req := httptest.NewRequest("PUT", path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("Token冒充人工采纳 %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "PUT", path, body, session)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"source":"feishu_session"`) {
		t.Fatalf("缺少人工来源 %d %s", w.Code, w.Body.String())
	}
}
