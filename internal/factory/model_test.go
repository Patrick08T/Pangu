package factory

import (
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/google/uuid"
)

func TestFactoryMessagePreservesExplicitModelOptions(t *testing.T) {
	var in Input
	raw := `{"id":"` + uuid.NewString() + `","revision":1,"mode":"build","message":"规划","connection_version":"` + uuid.NewString() + `","model":"chosen","reasoning_effort":"high","fast":false}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	var choice catalog.ConnectionChoice
	if err := json.Unmarshal([]byte(`{"id":"`+in.ConnectionVersion+`","model":"default-model","provider":"codex"}`), &choice); err != nil {
		t.Fatal(err)
	}
	d, _, err := Definition(Session{ProjectID: uuid.NewString()}, in, Context{Connections: []catalog.ConnectionChoice{choice}})
	if err != nil {
		t.Fatal(err)
	}
	node := d.Nodes[0]
	if node.Model != "chosen" || node.ReasoningEffort != "high" || node.Fast == nil || *node.Fast {
		t.Fatalf("工厂消息丢失显式模型选项: %+v", node)
	}
	if node.Executor != "eino" || node.Role != "" {
		t.Fatal("工厂扩大了执行权限")
	}
}

func TestFactoryRejectsRuntimeOptionsForAPIConnection(t *testing.T) {
	var in Input
	raw := `{"id":"` + uuid.NewString() + `","revision":1,"mode":"build","message":"规划","connection_version":"` + uuid.NewString() + `","model":"fixed","reasoning_effort":"high"}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	_, _, err := Definition(Session{ProjectID: uuid.NewString()}, in, Context{Connections: []catalog.ConnectionChoice{{ID: in.ConnectionVersion, Model: "fixed"}}})
	if err == nil {
		t.Fatal("普通 API 连接接受了 Codex 推理选项")
	}
}

func TestMessageEqualityPreservesThreeStateFast(t *testing.T) {
	for _, fast := range []string{"", `,"fast":false`, `,"fast":true`} {
		var first, retry Input
		raw := `{"model":"chosen","reasoning_effort":"high"` + fast + `}`
		if json.Unmarshal([]byte(raw), &first) != nil || json.Unmarshal([]byte(raw), &retry) != nil || !first.Equal(retry) {
			t.Fatal("相同消息重试比较失败", fast)
		}
		for _, other := range []string{"", `,"fast":false`, `,"fast":true`} {
			var changed Input
			_ = json.Unmarshal([]byte(`{"model":"chosen","reasoning_effort":"high"`+other+`}`), &changed)
			if first.Equal(changed) != (fast == other) {
				t.Fatal("Fast 默认、关闭、开启未分别处理", fast, other)
			}
		}
	}
}

func TestFactoryDefaultModelRejectsExplicitRuntimeOptions(t *testing.T) {
	fast := false
	for _, in := range []Input{{ReasoningEffort: "high"}, {Fast: &fast}, {Model: " spaced "}} {
		if in.validateModel() == nil {
			t.Fatal("接受了缺少具体模型的选项或非法模型名", in)
		}
	}
}
