package factory

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"testing"
)

func TestRetentionInvalidatesPreviouslyReadGenerationContext(t *testing.T) {
	ctx, ns, s := setup(t)
	in := prepareTest(t, ctx, ns, s)
	if err := Complete(ctx, ns, s.ID, in.ID, testReply(), ""); err != nil {
		t.Fatal(err)
	}
	s, err := Get(ctx, ns, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := in
	next.ID = uuid.NewString()
	next.Revision = s.Revision
	d, digest, err := Definition(*s, next, Context{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.factory_turns SET finished_at=now()-interval '31 days' WHERE id=$1", in.ID); err != nil {
		t.Fatal(err)
	}
	if err = Maintain(ctx, ns); err != nil {
		t.Fatal(err)
	}
	if _, err = Prepare(ctx, ns, s.ID, next, d, digest); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal("已清理上下文被重新发送", err)
	}
}

func TestRetentionClearsOldTitleButKeepsPublishedLinks(t *testing.T) {
	ctx, ns, s := setup(t)
	in := prepareTest(t, ctx, ns, s)
	if err := Complete(ctx, ns, s.ID, in.ID, testReply(), ""); err != nil {
		t.Fatal(err)
	}
	s, err := Get(ctx, ns, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(ctx, ns, s.ID, in.ID, s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.factory_turns SET finished_at=now()-interval '31 days' WHERE id=$1", in.ID); err != nil {
		t.Fatal(err)
	}
	if err = Maintain(ctx, ns); err != nil {
		t.Fatal(err)
	}
	s, err = Get(ctx, ns, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	turn := s.Turns[0]
	if s.Title != "历史对话（正文已清理）" || turn.Status != "expired" || turn.Reply != nil || turn.Input.Message != "正文已清理" {
		t.Fatal("旧内容仍可读取")
	}
	if turn.Result == nil || turn.Result.Agents[0].ID != result.Agents[0].ID || turn.Result.Agents[0].Spec.Agent != nil {
		t.Fatal("应用链接丢失或正文未清理")
	}
	again, err := Apply(ctx, ns, s.ID, in.ID, 1)
	if err != nil || again.Agents[0].ID != result.Agents[0].ID {
		t.Fatal("过期正文导致重复应用", err)
	}
}
