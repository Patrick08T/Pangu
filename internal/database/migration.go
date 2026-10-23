package database

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var source embed.FS

type migration struct{ name, sql string }

func (m migration) checksum() string { return fmt.Sprintf("%x", sha256.Sum256([]byte(m.sql))) }

func migrations() ([]migration, error) {
	entries, err := fs.ReadDir(source, "migrations")
	if err != nil {
		return nil, err
	}
	result := make([]migration, 0, len(entries))
	for _, entry := range entries {
		data, err := source.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}
		result = append(result, migration{name: entry.Name(), sql: string(data)})
	}
	return result, nil
}

func Migrate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	files, err := migrations()
	if err != nil {
		return err
	}
	return migrate(ctx, conn, files)
}

func migrate(ctx context.Context, conn *pgx.Conn, files []migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("无法开始数据库迁移事务")
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(721940815);
		CREATE TABLE IF NOT EXISTS public.pangu_schema_migrations (
		version text PRIMARY KEY, checksum text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now())`)
	if err != nil {
		return errors.New("无法获取迁移锁或创建迁移记录")
	}
	history, err := recorded(ctx, conn)
	if err != nil {
		return errors.New("无法读取迁移记录")
	}
	if err := validateHistory(files, history, false); err != nil {
		return err
	}
	for _, file := range files {
		if _, exists := history[file.name]; exists {
			continue
		}
		if _, err := tx.Exec(ctx, file.sql); err != nil {
			return fmt.Errorf("迁移 %s 失败，事务已回滚", file.name)
		}
		_, err := tx.Exec(ctx, "INSERT INTO public.pangu_schema_migrations(version, checksum) VALUES ($1,$2)", file.name, file.checksum())
		if err != nil {
			return errors.New("无法写入迁移记录")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("无法提交数据库迁移事务")
	}
	return nil
}

func recorded(ctx context.Context, conn *pgx.Conn) (map[string]string, error) {
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM public.pangu_schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			return nil, err
		}
		result[name] = checksum
	}
	return result, rows.Err()
}

func validateHistory(files []migration, history map[string]string, ready bool) error {
	known := make(map[string]bool, len(files))
	missing := false
	for _, file := range files {
		known[file.name] = true
		checksum, exists := history[file.name]
		if exists && checksum != file.checksum() {
			return fmt.Errorf("已应用的迁移 %s 被修改", file.name)
		}
		missing = missing || !exists
		if exists && missing {
			return errors.New("迁移历史存在缺口，禁止插入旧版本")
		}
	}
	for name := range history {
		if !known[name] {
			return errors.New("数据库包含当前程序未知的迁移，请使用匹配版本")
		}
	}
	if ready && missing {
		return errors.New("产品数据库有待执行迁移")
	}
	return nil
}
