package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestHandoffDraftCanSaveButCannotPublishOrRun(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	node := workflow.Node{ID: "agent", Executor: "codex", Instruction: "检查项目"}
	item, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "交接必填", Spec: Spec{Agent: &node}})
	if err != nil {
		t.Fatal("未完成草稿不能暂存", err)
	}
	if _, err = Publish(ctx, ns, item.ID, item.Revision); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "交接契约") {
		t.Fatalf("缺少契约仍可发布: %v", err)
	}
	if _, err = CompileDraft(ctx, ns, item.ID, item.Revision, project, "检查"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("缺少契约仍可试运行: %v", err)
	}
	if err = CheckDefinition(ctx, ns, workflow.Definition{ProjectID: project, Nodes: []workflow.Node{node}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("原始定义绕过契约: %v", err)
	}
	item.Spec.Agent.Handoff = testHandoff()
	item, err = Save(ctx, ns, item.ID, item.Revision, item.Name, item.Spec)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := CompileDraft(ctx, ns, item.ID, item.Revision, project, "检查")
	if err != nil || trial.Nodes[0].Handoff.Acceptance != testHandoff().Acceptance {
		t.Fatalf("完整草稿试运行契约丢失: %+v %v", trial, err)
	}
	v, err := Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, v.ID, project, "检查")
	if err != nil || d.Nodes[0].Handoff.Deliverables != testHandoff().Deliverables {
		t.Fatalf("版本契约丢失: %+v %v", d, err)
	}
}

func TestOldPublishedAgentRemainsReadableButCannotStartNewTask(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	item, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "历史助手", Spec: Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "历史职责"}}})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	id := uuid.NewString()
	body, _ := json.Marshal(item.Spec)
	_, err = conn.Exec(ctx, "INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) VALUES($1,$2,1,1,$3)", id, item.ID, body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadVersion(ctx, ns, id); err != nil {
		t.Fatal("历史版本不可读取", err)
	}
	if _, err = Compile(ctx, ns, id, project, "新任务"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "交接契约") {
		t.Fatalf("旧发布版本绕过必填契约: %v", err)
	}
}

func testHandoff() *workflow.HandoffContract {
	return &workflow.HandoffContract{Objective: "完成指定任务", Inputs: "本次输入和上游产物", Deliverables: "任务结果", Acceptance: "结果符合本次要求且说明验证证据", FailureHandling: "明确缺口，不得伪造完成"}
}
