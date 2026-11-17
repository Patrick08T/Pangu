package api

import (
	"bytes"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCatalogQueryAndArchiveRejectInvalidInput(t *testing.T) {
	for _, path := range []string{"/catalog?project_id=invalid", "/catalog?kind=invalid", "/catalog?offset=0&offset=1", "/catalog?unknown=yes", "/catalog/00000000-0000-0000-0000-000000000001/versions?offset=-1"} {
		w := httptest.NewRecorder()
		testAPI(&fakeBackend{}).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 {
			t.Errorf("接受错误查询 %s: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/catalog/00000000-0000-0000-0000-000000000001/archive", bytes.NewReader([]byte(`{}`)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testAPI(&fakeBackend{}).ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("省略 archived 不应默认为恢复: %d", w.Code)
	}
}

func TestCatalogArchiveFilterAcceptsOnlyExplicitBooleans(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		if _, err := catalogQuery(url.Values{"archived": {value}}, false); err != nil {
			t.Fatalf("归档筛选被拒绝：%s，%v", value, err)
		}
	}
	for _, value := range []string{"", "1", "FALSE", "all"} {
		if _, err := catalogQuery(url.Values{"archived": {value}}, false); err == nil {
			t.Fatalf("接受错误归档筛选：%q", value)
		}
	}
	if _, err := catalogQuery(url.Values{"archived": {"true"}}, true); err == nil {
		t.Fatal("版本列表不应接受配置归档筛选")
	}
}
