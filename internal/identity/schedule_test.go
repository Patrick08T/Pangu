package identity_test

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"os"
	"testing"
	"time"
)

func TestDisablePausesOwnedSchedulesAndPreservesAcceptedFire(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	ns, project, adminID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,$2,'test','test','main','worker')`, project, ns)
	if err != nil {
		t.Fatal(err)
	}
	for id, admin := range map[string]bool{adminID: true, userID: false} {
		_, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,'test','test',$4,$3)`, id, ns, admin, id)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.memberships(user_id,project_id,role) VALUES($1,$2,'operator')`, userID, project)
	if err != nil {
		t.Fatal(err)
	}
	actor := identity.Actor{ID: userID, Namespace: ns}
	userCtx := identity.WithActor(ctx, actor)
	d := workflow.Definition{ProjectID: project, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "test", Instruction: "test"}}}
	p, err := schedule.Create(userCtx, ns, workflow.Cron{Name: "daily", Expression: "* * * * *", Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	if p.CreatedBy != userID {
		t.Fatal("计划未记录真实创建者")
	}
	slot := time.Now().UTC().Truncate(time.Minute)
	fire, err := schedule.Claim(ctx, ns, p.ID, p.Revision, slot, func(context.Context, string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	if fire.Status != "pending" || fire.Snapshot.CreatedBy != userID {
		t.Fatalf("触发缺少归属: %+v", fire)
	}
	if err = identity.Disable(ctx, identity.Actor{ID: adminID, Namespace: ns}, userID); err != nil {
		t.Fatal(err)
	}
	p, err = schedule.Get(ctx, ns, p.ID)
	if err != nil || !p.Paused {
		t.Fatalf("禁用未暂停: %+v %v", p, err)
	}
	next, err := schedule.Claim(ctx, ns, p.ID, p.Revision, slot.Add(time.Minute), func(context.Context, string) (bool, error) { return false, nil })
	if err != nil || next.Status != "skipped" {
		t.Fatalf("禁用后新触发: %+v %v", next, err)
	}
	accepted, err := submission.Prepare(ctx, ns, "cron", p.ID, fire.Snapshot)
	if err != nil || accepted.CreatedBy != userID {
		t.Fatalf("已接受触发不能继续: %+v %v", accepted, err)
	}
	if _, err = schedule.Save(userCtx, ns, p.ID, p.Revision, workflow.Cron{Name: p.Name, Expression: p.Expression, Workflow: p.Workflow}, false); err == nil {
		t.Fatal("禁用身份恢复了计划")
	}
}
