package execution

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestAttemptPaginationRejectsAmbiguity(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=101", "offset=-1", "limit=2&limit=3", "node=secret", "limit="} {
		values, _ := url.ParseQuery(query)
		if _, _, err := ParsePage(values); err == nil {
			t.Fatalf("接受非法分页 %q", query)
		}
	}
	offset, limit, err := ParsePage(url.Values{"offset": {"10"}, "limit": {"5"}})
	if err != nil || offset != 10 || limit != 5 {
		t.Fatalf("合法分页失败: %d %d %v", offset, limit, err)
	}
}

func TestAttemptPaginationDoesNotHideActiveExecution(t *testing.T) {
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
	run := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt,started_at,finished_at,status)
		SELECT $1,gen_random_uuid(),'node',i,now()+i*interval '1 millisecond',CASE WHEN i<101 THEN now() END,
		CASE WHEN i<101 THEN 'succeeded' ELSE 'running' END FROM generate_series(1,101) AS i`, run)
	if err != nil {
		t.Fatal(err)
	}
	page, err := ListAttempts(ctx, run, 0, 20)
	if err != nil || len(page.Items) != 20 || page.NextOffset == nil || *page.NextOffset != 20 {
		t.Fatalf("分页异常: %+v %v", page, err)
	}
	active, err := Active(ctx, run)
	if err != nil || active != 1 {
		t.Fatalf("分页隐藏了活动尝试: %d %v", active, err)
	}
	last, err := ListAttempts(ctx, run, 100, 20)
	if err != nil || len(last.Items) != 1 || last.NextOffset != nil || last.Items[0].Status != "running" {
		t.Fatalf("最后一页异常: %+v %v", last, err)
	}
	if _, err := AttemptByID(ctx, uuid.NewString(), last.Items[0].NodeRunID, 101); err == nil {
		t.Fatal("跨运行读取了节点证据")
	}
}

func TestCancellationReconciliationRotatesBeyondFirstPage(t *testing.T) {
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
	namespace := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.run_cancellations(run_id,namespace) SELECT gen_random_uuid(),$1 FROM generate_series(1,101)`, namespace)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Pending(ctx, namespace)
	if err != nil || len(first) != 100 {
		t.Fatalf("首页异常: %d %v", len(first), err)
	}
	seen := map[string]bool{}
	for _, id := range first {
		seen[id] = true
		if err := Checked(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	next, err := Pending(ctx, namespace)
	if err != nil || len(next) == 0 || seen[next[0]] {
		t.Fatalf("第一页失败导致后续请求饥饿: %v", err)
	}
}
