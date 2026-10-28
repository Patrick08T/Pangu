package api

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestSharedChoicesRejectProjectAndInvalidPagination(t *testing.T) {
	h, _, project, _, token := authorizationAPI(t)
	for _, tc := range []struct {
		query  string
		status int
	}{{"", 200}, {"offset=0", 200}, {"offset=-1", 400}, {"project_id=" + project, 400}} {
		w := callAuthorized(h, "GET", "/runnables?"+tc.query, "", token)
		if w.Code != tc.status {
			t.Errorf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
	}
}

func TestRunnableChoicesAreWorkspaceScoped(t *testing.T) {
	h, _, project, other, token := authorizationAPI(t)
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"project_id=" + project, 400}, {"project_id=" + other, 400},
		{"", 200}, {"project_id=invalid", 400}, {"project_id=" + project + "&offset=-1", 400},
		{"project_id=" + project + "&secret=true", 400},
	} {
		w := callAuthorized(h, "GET", "/runnables?"+tc.query, "", token)
		if w.Code != tc.status {
			t.Errorf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
	}
}

func TestSharedConnectionChoicesOmitCredentialDetails(t *testing.T) {
	h, ns, project, other, token := authorizationAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project)
	if err != nil {
		t.Fatal(err)
	}
	var selected, object string
	for _, p := range []string{project, other} {
		id, version := uuid.NewString(), uuid.NewString()
		_, err = conn.Exec(ctx, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,'connection','可绑定模型','{"connection":{"model":"test","base_url":"https://private.example","secret_ref":"PANGU_SECRET_PRIVATE"}}')`, id, ns)
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Exec(ctx, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) SELECT $1,id,1,1,spec FROM pangu.catalog_objects WHERE id=$2`, version, id)
		if err != nil {
			t.Fatal(err)
		}
		if p == project {
			selected, object = version, id
		}
	}
	path := "/connections/choices"
	w := callAuthorized(h, "GET", path, "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), selected) || strings.Contains(w.Body.String(), "private.example") || strings.Contains(w.Body.String(), "secret_ref") {
		t.Fatalf("选择器泄漏或不可用: %d %s", w.Code, w.Body.String())
	}
	if w = callAuthorized(h, "GET", "/connections/choices?project_id="+other, "", token); w.Code != 400 {
		t.Fatalf("拒绝旧项目参数: %d", w.Code)
	}
	if w = callAuthorized(h, "GET", "/catalog/"+object, "", token); w.Code != 200 {
		t.Fatalf("共享连接配置不可读: %d", w.Code)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.catalog_objects SET archived=true WHERE id=$1", object); err != nil {
		t.Fatal(err)
	}
	w = callAuthorized(h, "GET", path, "", token)
	if w.Code != 200 || strings.Contains(w.Body.String(), selected) {
		t.Fatalf("归档仍可选择: %d %s", w.Code, w.Body.String())
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET enabled=false WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w = callAuthorized(h, "GET", path, "", token); w.Code != 200 {
		t.Fatalf("项目撤权影响了共享目录读取: %d", w.Code)
	}
}
