package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestShippedFactoryTemplatesAreValidAndProjectFree(t *testing.T) {
	for _, name := range []string{"product-research", "product-competition", "product-planning", "product-design", "code-change", "code-inspection"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var item struct {
			Format string `json:"format"`
			NewObject
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&item); err != nil {
			t.Fatal(err)
		}
		if item.Format != "pangu.catalog.v1" || item.Kind != "template" || item.ProjectID != "" || item.Spec.TemplateVersion != "" {
			t.Fatalf("模板包含项目绑定: %+v", item)
		}
		if err := item.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if name == "code-inspection" && !item.Spec.Agent.ReadOnly {
			t.Fatal("只读巡检模板未启用沙箱")
		}
	}
}
