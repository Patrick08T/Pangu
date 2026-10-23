package execution

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestLatestCategoriesIncludeNodesBeyondFirstPage(t *testing.T) {
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
	run, node := uuid.NewString(), uuid.NewString()
	for i := 1; i <= 25; i++ {
		_, err = conn.Exec(ctx, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt,status,category) VALUES($1,$2,'first',$3,'failed','timeout')`, run, node, i)
		if err != nil {
			t.Fatal(err)
		}
	}
	later := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt,status,category) VALUES($1,$2,'later',1,'failed','authentication'),($1,$3,'success',26,'succeeded','')`, run, later, node)
	if err != nil {
		t.Fatal(err)
	}
	categories, err := LatestCategories(ctx, run)
	if err != nil || len(categories) != 2 || categories[node] != "" || categories[later] != "authentication" {
		t.Fatalf("分类遗漏或残留旧错误: %+v %v", categories, err)
	}
}
