package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestFactoryAPIStoresAndRetriesModelSelection(t *testing.T) {
	_, project := trialObject(t)
	ctx := context.Background()
	c, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "connection", Name: "模型选择", Spec: catalog.Spec{Connection: &catalog.Connection{Provider: "codex", Model: "default"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, "api-test", c.ID, c.Revision)
	if err != nil {
		t.Fatal(err)
	}
	s, err := factory.Create(ctx, "api-test", project)
	if err != nil {
		t.Fatal(err)
	}
	h := New(&factoryTestBackend{}, testAccess{})
	fast := false
	in := factory.Input{ID: uuid.NewString(), Revision: 1, Message: "创建助手", Mode: "build", ConnectionVersion: v.ID, Model: "chosen", ReasoningEffort: "high", Fast: &fast}
	path := "/factory/sessions/" + s.ID + "/messages"
	var first submission.Record
	for i := 0; i < 2; i++ {
		w := factoryRequest(h, "POST", path, in)
		var got submission.Record
		if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &got) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		if i == 0 {
			first = got
		} else if first.ID != got.ID {
			t.Fatal("重试重复提交")
		}
	}
	saved, err := factory.Get(ctx, "api-test", s.ID)
	if err != nil || len(saved.Turns) != 1 || !saved.Turns[0].Input.Equal(in) {
		t.Fatal("历史配置丢失", err)
	}
	record, err := submission.Get(ctx, "api-test", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	node := record.Snapshot.Workflow.Nodes[0]
	if node.Model != in.Model || node.ReasoningEffort != in.ReasoningEffort || node.Fast == nil || *node.Fast {
		t.Fatal("运行快照丢失配置", node)
	}
	for _, change := range []func(*factory.Input){func(v *factory.Input) { v.Model = "other" }, func(v *factory.Input) { v.ReasoningEffort = "low" }, func(v *factory.Input) { v.Fast = nil }} {
		changed := in
		change(&changed)
		if w := factoryRequest(h, "POST", path, changed); w.Code != 409 {
			t.Fatal("同 ID 参数变更未冲突", w.Code, w.Body.String())
		}
	}
}
