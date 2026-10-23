package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
)

func TestCatalogImportExportHasExplicitFormatAndNewIdentity(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := testAPI(&fakeBackend{})
	raw := []byte(`{"format":"pangu.catalog.v1","kind":"template","name":"只读巡检","spec":{"agent":{"id":"inspect","executor":"codex","instruction":"只读检查","read_only":true}}}`)
	invoke := func(method, path string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	first := invoke("POST", "/catalog/import", raw)
	if first.Code != 201 {
		t.Fatalf("导入失败: %d %s", first.Code, first.Body.String())
	}
	var object catalog.Object
	if err := json.Unmarshal(first.Body.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	exported := invoke("GET", "/catalog/"+object.ID+"/export", nil)
	if exported.Code != 200 {
		t.Fatal(exported.Body.String())
	}
	second := invoke("POST", "/catalog/import", exported.Body.Bytes())
	var clone catalog.Object
	_ = json.Unmarshal(second.Body.Bytes(), &clone)
	if second.Code != 201 || clone.ID == object.ID || clone.Spec.Agent == nil || !clone.Spec.Agent.ReadOnly {
		t.Fatalf("导入覆盖原对象或丢失策略: %s", second.Body.String())
	}
	invalid := bytes.Replace(raw, []byte("pangu.catalog.v1"), []byte("future.v2"), 1)
	if got := invoke("POST", "/catalog/import", invalid); got.Code != 400 {
		t.Fatalf("未知格式没有拒绝: %d", got.Code)
	}
}
