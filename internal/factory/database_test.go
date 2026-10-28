package factory

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"os"
	"testing"
)

func setup(t *testing.T) (context.Context, string, *Session) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ns := uuid.NewString()
	p, err := workspace.Register(ctx, ns, workspace.Project{Name: "对话工厂", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Create(ctx, ns, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, ns, s
}

func prepareTest(t *testing.T, ctx context.Context, ns string, s *Session) Input {
	t.Helper()
	in := Input{ID: uuid.NewString(), Revision: s.Revision, Message: "生成写作助手", Mode: "build", ConnectionVersion: uuid.NewString()}
	d, digest, err := Definition(*s, in, Context{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Prepare(ctx, ns, s.ID, in, d, digest); err != nil {
		t.Fatal(err)
	}
	return in
}

func testReply() string {
	b := catalog.Bundle{Name: "写作助手", Agents: []catalog.AgentChange{{Key: "writer", Name: "写作助手", Node: &workflow.Node{ID: "writer", Executor: "codex", Instruction: "根据要求写作", ReadOnly: true, Handoff: &workflow.HandoffContract{Objective: "写作", Inputs: "用户要求", Deliverables: "正文", Acceptance: "满足要求", FailureHandling: "明确缺口"}}}}}
	data, _ := json.Marshal(Reply{Message: "请预览", Proposal: &b})
	return string(data)
}

func TestConversationApplyIsAtomicIdempotentAndDoesNotSubmitBusinessRun(t *testing.T) {
	ctx, ns, s := setup(t)
	in := prepareTest(t, ctx, ns, s)
	if err := Complete(ctx, ns, s.ID, in.ID, testReply(), ""); err != nil {
		t.Fatal(err)
	}
	s, err := Get(ctx, ns, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Apply(ctx, ns, s.ID, in.ID, s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Apply(ctx, ns, s.ID, in.ID, s.Revision)
	if err != nil || again.Agents[0].ID != out.Agents[0].ID {
		t.Fatal("重复创建", err)
	}
	items, err := catalog.List(ctx, ns, "", 0)
	if err != nil || len(items) != 1 {
		t.Fatal("对象数异常", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var count int
	err = conn.QueryRow(ctx, "SELECT count(*) FROM pangu.submissions WHERE namespace=$1", ns).Scan(&count)
	if err != nil || count != 1 {
		t.Fatal("创建触发了业务执行", count, err)
	}
}

func TestConversationRejectsStaleAndInvalidCandidates(t *testing.T) {
	ctx, ns, s := setup(t)
	in := prepareTest(t, ctx, ns, s)
	if _, err := Apply(ctx, ns, s.ID, in.ID, 2); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal("未完成候选可应用", err)
	}
	if err := Complete(ctx, ns, s.ID, in.ID, `{"message":"ok","proposal":{"agents":[]}}`, ""); err != nil {
		t.Fatal(err)
	}
	s, err := Get(ctx, ns, s.ID)
	if err != nil || s.Turns[0].Status != "failed" {
		t.Fatal("无效回复未保留失败状态", err)
	}
	if _, err = Apply(ctx, ns, s.ID, in.ID, s.Revision); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal("无效候选被应用", err)
	}
	if _, err = Prepare(ctx, ns, s.ID, Input{ID: uuid.NewString(), Revision: 1}, workflow.Definition{}, ""); err == nil {
		t.Fatal("旧修订被接受")
	}
}
