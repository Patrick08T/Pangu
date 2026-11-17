package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestModelOptionsPersistExplicitFastOff(t *testing.T) {
	var node Node
	if err := json.Unmarshal([]byte(`{"id":"a","executor":"codex","model":"test","instruction":"实现","reasoning_effort":"high","fast":false}`), &node); err != nil {
		t.Fatal(err)
	}
	if err := node.Validate(); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(node)
	if !strings.Contains(string(data), `"fast":false`) || !strings.Contains(string(data), `"reasoning_effort":"high"`) {
		t.Fatalf("运行配置在固定版本序列化时丢失：%s", data)
	}
}

func TestModelOptionsRejectMissingModelAndInvalidEffort(t *testing.T) {
	for _, raw := range []string{
		`{"id":"a","executor":"codex","instruction":"实现","fast":true}`,
		`{"id":"a","executor":"codex","model":"test","instruction":"实现","reasoning_effort":"invalid"}`,
	} {
		var node Node
		if err := json.Unmarshal([]byte(raw), &node); err != nil {
			t.Fatal(err)
		}
		if err := node.Validate(); err == nil {
			t.Errorf("接受非法模型运行选项：%s", raw)
		}
	}
}
