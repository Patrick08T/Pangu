package agent

import "testing"

func TestCodexModelChoicesFilterSortAndValidate(t *testing.T) {
	raw := []byte(`{"models":[{"slug":"b","display_name":"模型 B","visibility":"list","priority":2},{"slug":"hidden","visibility":"hide"},{"slug":"a","visibility":"list","priority":1}]}`)
	items, err := codexModelChoices(raw)
	if err != nil || len(items) != 2 || items[0].ID != "a" || items[0].Name != "a" || items[1].Name != "模型 B" {
		t.Fatalf("模型目录过滤排序错误：%v %v", items, err)
	}
	for _, raw := range []string{`{}`, `{"models":[{"slug":"","visibility":"list"}]}`, `{"models":[{"slug":"a\nsecret","visibility":"list"}]}`, `{"models":false}`} {
		if _, err := codexModelChoices([]byte(raw)); err == nil {
			t.Fatalf("接受无效模型目录：%s", raw)
		}
	}
}
