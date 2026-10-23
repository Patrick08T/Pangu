package database

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func connect(ctx context.Context) (*pgx.Conn, error) {
	value := os.Getenv("PANGU_DATABASE_URL")
	if value == "" {
		return nil, errors.New("未配置 PANGU_DATABASE_URL")
	}
	config, err := pgx.ParseConfig(value)
	if err != nil {
		return nil, errors.New("PANGU_DATABASE_URL 格式错误")
	}
	switch config.Database {
	case "", "postgres", "hatchet", "template0", "template1":
		return nil, errors.New("必须使用独立的 Pangu 产品数据库")
	}
	config.ConnectTimeout = 3 * time.Second
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, errors.New("产品数据库连接失败，请检查连接配置及服务")
	}
	var privileged bool
	err = conn.QueryRow(ctx, "SELECT rolsuper OR rolcreatedb OR rolcreaterole FROM pg_roles WHERE rolname=current_user").Scan(&privileged)
	if err != nil || privileged {
		_ = conn.Close(ctx)
		return nil, errors.New("产品数据库必须使用无超级用户、建库和建角色权限的独立账号")
	}
	return conn, nil
}

// Open 每次读取原始连接配置并检查数据库账号权限。
func Open(ctx context.Context) (*pgx.Conn, error) { return connect(ctx) }

// Check 检查连接和迁移版本；每次读取连接来源，不缓存配置或就绪结果。
func Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
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
	history, err := recorded(ctx, conn)
	if err != nil {
		return errors.New("产品数据库尚未迁移或版本记录不可读取")
	}
	return validateHistory(files, history, true)
}
