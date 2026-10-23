package database

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestPostgresMigrations(t *testing.T) {
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置独立的 PANGU_TEST_DATABASE_URL")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var name string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil || name != "pangu_test" {
		t.Fatal("集成测试仅允许使用 pangu_test 数据库")
	}
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS pangu CASCADE; DROP TABLE IF EXISTS public.pangu_schema_migrations"); err != nil {
		t.Fatal(err)
	}
	if err := Check(ctx); err == nil {
		t.Fatal("unmigrated database reported ready")
	}
	broken := []migration{{name: "bad.sql", sql: "CREATE SCHEMA pangu; SELECT invalid_column;"}}
	if err := migrate(ctx, conn, broken); err == nil {
		t.Fatal("broken migration succeeded")
	}
	var schema *string
	if err := conn.QueryRow(ctx, "SELECT to_regnamespace('pangu')::text").Scan(&schema); err != nil || schema != nil {
		t.Fatal("failed migration was not rolled back")
	}
	for range 2 {
		if err := Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "UPDATE public.pangu_schema_migrations SET checksum='changed' WHERE version='0001_namespace.sql'"); err != nil {
		t.Fatal(err)
	}
	files, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "UPDATE public.pangu_schema_migrations SET checksum=$1 WHERE version=$2", files[0].checksum(), files[0].name)
	if err := Migrate(ctx); err == nil {
		t.Fatal("changed applied migration was accepted")
	}
}

func TestConcurrentMigrations(t *testing.T) {
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置独立的 PANGU_TEST_DATABASE_URL")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if conn.Config().Database != "pangu_test" {
		t.Fatal("集成测试仅允许使用 pangu_test 数据库")
	}
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			if err := Migrate(ctx); err != nil {
				t.Errorf("并发迁移失败: %v", err)
			}
		})
	}
	group.Wait()
	if err := Check(ctx); err != nil {
		t.Fatal(err)
	}
}
