package evaluation

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"testing"
)

func TestEvaluationCaseRequiresExplicitRetentionAndExpiresBody(t *testing.T) {
	ns, project, _, user := evaluationFixture(t)
	ctx := context.Background()
	input := CaseInput{Name: "来源核查", Input: "比较两份公开资料", Expected: "指出冲突并提供来源", Criteria: []string{"事实与推断分开"}}
	if _, err := CreateCase(ctx, ns, project, user, input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("未经明确确认保存输入: %v", err)
	}
	input.RetainInput = true
	saved, err := CreateCase(ctx, ns, project, user, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadCase(ctx, "foreign", saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨空间读取: %v", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.evaluation_cases SET expires_at=now()-interval '1 second' WHERE id=$1", saved.ID); err != nil {
		t.Fatal(err)
	}
	expired, err := ReadCase(ctx, ns, saved.ID)
	if err != nil || !expired.PayloadExpired || expired.Input != "" || expired.Expected != "" || len(expired.Criteria) != 0 {
		t.Fatalf("读取过期正文: %+v %v", expired, err)
	}
	if err = Maintain(ctx, ns); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err = conn.QueryRow(ctx, "SELECT input FROM pangu.evaluation_cases WHERE id=$1", saved.ID).Scan(&raw); err != nil || raw != "" {
		t.Fatalf("磁盘正文未清理: %s %v", raw, err)
	}
}
