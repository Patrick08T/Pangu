package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/factory"
)

func TestFactoryHistoryListsCurrentProjectAndSupportsPagination(t *testing.T) {
	h, ns, project, other, token := authorizationAPI(t)
	owned, err := factory.Create(context.Background(), ns, project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = factory.Create(context.Background(), ns, other); err != nil {
		t.Fatal(err)
	}
	w := callAuthorized(h, "GET", "/factory/sessions?project_id="+project+"&offset=0", "", token)
	var result page[factory.Session]
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Items) != 1 || result.Items[0].ID != owned.ID {
		t.Fatalf("历史列表未返回当前项目已有对话：%d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", "/factory/sessions?project_id="+project+"&offset=1", "", token)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Items) != 0 {
		t.Fatalf("分页没有生效：%d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", "/factory/sessions?project_id="+other, "", token)
	if w.Code != 403 {
		t.Fatalf("跨项目查询未保持权限隔离：%d", w.Code)
	}
}

func TestProjectListQueriesKeepEndpointPermissionBoundaries(t *testing.T) {
	h, _, project, _, token := authorizationAPI(t)
	for path, expected := range map[string]int{
		"/factory/sessions": 200, "/deliveries": 200, "/evaluations/cases": 200,
		"/crons": 200, "/github/hooks": 403, "/audit": 403,
	} {
		w := callAuthorized(h, "GET", path+"?project_id="+project+"&offset=0", "", token)
		if w.Code != expected {
			t.Errorf("%s 参数校验或权限边界错误：%d %s", path, w.Code, w.Body.String())
		}
		for _, extra := range []string{"&kind=workflow", "&archived=false", "&project_id=" + project, "&offset=1", "&unknown=yes"} {
			w = callAuthorized(h, "GET", path+"?project_id="+project+"&offset=0"+extra, "", token)
			if w.Code != 400 {
				t.Errorf("%s 接受非法查询 %s：%d", path, extra, w.Code)
			}
		}
	}
}

func TestFactoryHistoryRejectsInvalidProjectAndOffset(t *testing.T) {
	h, _, project, _, token := authorizationAPI(t)
	queries := []string{"", "project_id=", "project_id=invalid", "project_id=" + project + "&offset=-1", "project_id=" + project + "&offset=", "project_id=" + project + "&offset=abc"}
	for _, query := range queries {
		w := callAuthorized(h, "GET", "/factory/sessions?"+query, "", token)
		if w.Code != 400 {
			t.Errorf("接受非法查询 %s：%d", query, w.Code)
		}
	}
}

func TestProjectListQueriesPreserveOptionalProject(t *testing.T) {
	h, _, _, _, token := authorizationAPI(t)
	for path, expected := range map[string]int{"/crons": 200, "/audit": 403} {
		w := callAuthorized(h, "GET", path+"?offset=0", "", token)
		if w.Code != expected {
			t.Errorf("%s 省略项目后行为改变：%d %s", path, w.Code, w.Body.String())
		}
		for _, query := range []string{"project_id=", "project_id=invalid", "offset=-1", "offset=99999999999999999999"} {
			w = callAuthorized(h, "GET", path+"?"+query, "", token)
			if w.Code != 400 {
				t.Errorf("%s 接受非法查询 %s：%d", path, query, w.Code)
			}
		}
	}
}
