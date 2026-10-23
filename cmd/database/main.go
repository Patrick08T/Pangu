package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := provision(ctx); err != nil {
		log.Fatal(err)
	}
	log.Print("独立产品数据库与账号已就绪")
}

func target(value string) (*pgx.ConnConfig, error) {
	config, err := pgx.ParseConfig(value)
	if err != nil || value == "" {
		return nil, errors.New("产品数据库连接格式错误")
	}
	if config.User == "" || config.User == "postgres" || config.Password == "" {
		return nil, errors.New("产品数据库必须使用独立账号和非空密码")
	}
	switch config.Database {
	case "", "hatchet", "postgres", "template0", "template1":
		return nil, errors.New("禁止初始化系统或 Hatchet 数据库")
	}
	return config, nil
}

func provision(ctx context.Context) error {
	config, err := target(os.Getenv("PANGU_DATABASE_URL"))
	if err != nil {
		return err
	}
	adminURL := os.Getenv("PANGU_DATABASE_ADMIN_URL")
	admin, err := pgx.ParseConfig(adminURL)
	if err != nil || adminURL == "" || admin.Host != config.Host || admin.Port != config.Port {
		return errors.New("管理员连接必须显式配置并指向同一 PostgreSQL 实例")
	}
	conn, err := pgx.ConnectConfig(ctx, admin)
	if err != nil {
		return errors.New("无法连接 PostgreSQL 管理端")
	}
	defer conn.Close(context.Background())
	if err := createRole(ctx, conn, config); err != nil {
		return err
	}
	if err := createDatabase(ctx, conn, config); err != nil {
		return err
	}
	app, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return errors.New("产品账号无法连接，请核对已有账号密码；不会自动修改已有密码")
	}
	defer app.Close(context.Background())
	var privileged bool
	err = app.QueryRow(ctx, "SELECT rolsuper OR rolcreatedb OR rolcreaterole FROM pg_roles WHERE rolname=current_user").Scan(&privileged)
	if err != nil || privileged {
		return errors.New("产品账号权限过高或无法验证")
	}
	return nil
}

func createRole(ctx context.Context, conn *pgx.Conn, config *pgx.ConnConfig) error {
	var exists bool
	err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", config.User).Scan(&exists)
	if err != nil {
		return errors.New("无法查询产品角色")
	}
	if exists {
		return nil
	}
	var sql string
	err = conn.QueryRow(ctx, `SELECT format('CREATE ROLE %I LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L', $1::text, $2::text)`, config.User, config.Password).Scan(&sql)
	if err == nil {
		_, err = conn.Exec(ctx, sql)
	}
	if err != nil {
		return errors.New("无法创建产品角色")
	}
	return nil
}

func createDatabase(ctx context.Context, conn *pgx.Conn, config *pgx.ConnConfig) error {
	var owner string
	err := conn.QueryRow(ctx, "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=$1", config.Database).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		statement := fmt.Sprintf("CREATE DATABASE %s OWNER %s", pgx.Identifier{config.Database}.Sanitize(), pgx.Identifier{config.User}.Sanitize())
		if _, err := conn.Exec(ctx, statement); err != nil {
			return errors.New("无法创建产品数据库")
		}
	} else if err != nil || owner != config.User {
		return errors.New("已有数据库不属于产品账号，拒绝修改")
	}
	_, err = conn.Exec(ctx, "REVOKE ALL ON DATABASE "+pgx.Identifier{config.Database}.Sanitize()+" FROM PUBLIC")
	if err != nil {
		return errors.New("无法限制产品数据库的公共权限")
	}
	return nil
}
