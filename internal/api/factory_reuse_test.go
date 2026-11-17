package api

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/google/uuid"
)

func factoryReuseInput(t *testing.T) factory.Input {
	t.Helper()
	ctx := context.Background()
	item, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "connection", Name: "复用测试连接", Spec: catalog.Spec{Connection: &catalog.Connection{Provider: "codex"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, "api-test", item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return factory.Input{ID: uuid.NewString(), Revision: 1, Message: "使用已有角色创建流程", Mode: "build", ConnectionVersion: v.ID}
}

func factoryReusePayload(t *testing.T, project string, in factory.Input) ([]catalog.RunnableChoice, []catalog.Version) {
	t.Helper()
	h := handler{backend: &factoryTestBackend{}}
	data, err := h.factoryContext(context.Background(), project, in)
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := factory.Definition(factory.Session{ProjectID: project}, in, *data)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Context struct {
			Agents     []catalog.RunnableChoice `json:"agents"`
			Components []catalog.Version        `json:"components"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(d.Input), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Context.Agents, payload.Context.Components
}

func TestFactoryBuildLoadsReusableRolesAndReloadsChanges(t *testing.T) {
	item, project := trialObject(t)
	ctx := context.Background()
	v, err := catalog.Publish(ctx, "api-test", item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	in := factoryReuseInput(t)
	for _, state := range []string{"published", "updated", "archived"} {
		if state == "updated" {
			item.Spec.Agent.Instruction = "更新后的完整职责"
			item, err = catalog.Save(ctx, "api-test", item.ID, item.Revision, "更新角色", item.Spec)
			if err != nil {
				t.Fatal(err)
			}
			v, err = catalog.Publish(ctx, "api-test", item.ID, item.Revision)
			if err != nil {
				t.Fatal(err)
			}
		} else if state == "archived" {
			if err := catalog.Archive(ctx, "api-test", item.ID, true); err != nil {
				t.Fatal(err)
			}
		}
		agents, _ := factoryReusePayload(t, project, in)
		found := false
		for _, a := range agents {
			if a.Version.ObjectID == item.ID {
				found = true
				if a.Name != item.Name || !reflect.DeepEqual(a.Version, *v) {
					t.Fatalf("角色名称、固定版本或完整契约丢失: %s", state)
				}
			}
		}
		if found != (state != "archived") {
			t.Fatalf("模型未读取当前角色仓库: %s", state)
		}
	}
}

func TestFactoryReusableRolesPreserveTargetPinnedVersions(t *testing.T) {
	item, project := trialObject(t)
	ctx := context.Background()
	old, err := catalog.Publish(ctx, "api-test", item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "workflow", Name: "固定旧角色版本", Spec: catalog.Spec{Flow: &catalog.Flow{Nodes: []catalog.Binding{{ID: "check", AgentVersion: old.ID}}}}})
	if err != nil {
		t.Fatal(err)
	}
	item.Spec.Agent.Instruction = "新版本职责"
	item, err = catalog.Save(ctx, "api-test", item.ID, item.Revision, item.Name, item.Spec)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := catalog.Publish(ctx, "api-test", item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	in := factoryReuseInput(t)
	in.Mode, in.TargetID, in.TargetRevision = "modify", flow.ID, flow.Revision
	agents, components := factoryReusePayload(t, project, in)
	if len(components) != 1 || !reflect.DeepEqual(components[0], *old) {
		t.Fatal("读取角色仓库覆盖了目标的固定版本")
	}
	for _, a := range agents {
		if a.Version.ID == latest.ID {
			return
		}
	}
	t.Fatal("可复用角色遗漏最新发布版本")
}
