package usage

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestAbsentBudgetDoesNotImposeDefaultCallLimits(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ns, run := uuid.NewString(), uuid.NewString()
	project, err := workspace.Register(ctx, ns, workspace.Project{Name: "无隐式预算", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := submission.Prepare(ctx, ns, "submit", "unlimited", submission.Snapshot{Workflow: workflow.Definition{ProjectID: project.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = submission.Dispatch(ctx, ns, item.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	node := uuid.NewString()
	ctx = WithRun(ctx, run, "agent", node, 1)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for kind, count := range map[string]int{"model": 1056, "search": 960, "codex": 96} {
		_, err = conn.Exec(ctx, `INSERT INTO pangu.usage_calls(id,run_id,node_id,node_run_id,attempt,kind,model)
   SELECT gen_random_uuid(),$1,'agent',$2,1,$3,'test' FROM generate_series(1,$4::int)`, run, node, kind, count)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Reserve(ctx, kind, "test", nil); err != nil {
			t.Fatalf("%s 仍施加默认上限: %v", kind, err)
		}
	}
}
