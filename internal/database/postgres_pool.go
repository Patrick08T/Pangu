package database

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var maintenancePool struct {
	sync.Mutex
	key  [32]byte
	pool *pgxpool.Pool
}

// Acquire 为短时后台查询复用连接；调用方必须 Release，不用于会话锁或长时间外部操作。
func Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pool, err := currentPool(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("后台数据库连接失败，请检查连接配置及账号权限")
	}
	return conn, nil
}

func currentPool(ctx context.Context) (*pgxpool.Pool, error) {
	maintenancePool.Lock()
	defer maintenancePool.Unlock()
	value := os.Getenv("PANGU_DATABASE_URL")
	config, err := pgxpool.ParseConfig(value)
	if value == "" {
		err = errors.New("未配置 PANGU_DATABASE_URL")
	} else if err != nil {
		err = errors.New("PANGU_DATABASE_URL 格式错误")
	} else {
		err = validateConfig(config.ConnConfig)
	}
	if err != nil {
		replacePool(nil, [32]byte{})
		return nil, err
	}
	key := poolKey(value, config.ConnConfig)
	if maintenancePool.pool != nil && key == maintenancePool.key {
		return maintenancePool.pool, nil
	}
	config.MaxConns, config.MinConns, config.MinIdleConns = 4, 0, 0
	config.MaxConnIdleTime = time.Minute
	config.PrepareConn = func(ctx context.Context, conn *pgx.Conn) (bool, error) {
		err := validateRole(ctx, conn)
		return err == nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New("无法创建后台数据库连接池")
	}
	replacePool(pool, key)
	return pool, nil
}

func poolKey(value string, config *pgx.ConnConfig) [32]byte {
	environment := []string{}
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "PG") {
			environment = append(environment, item)
		}
	}
	slices.Sort(environment)
	// 纳入解析后的来源，避免 service/passfile 更新后仍使用旧连接。
	values := []any{value, environment, config.Host, config.Port, config.Database, config.User, config.Password, config.RuntimeParams}
	if file := os.Getenv("PGSERVICEFILE"); file != "" {
		body, _ := os.ReadFile(file)
		values = append(values, body)
	}
	for _, fallback := range config.Fallbacks {
		values = append(values, fallback.Host, fallback.Port)
	}
	body, _ := json.Marshal(values)
	return sha256.Sum256(body)
}

// 调用方持有互斥锁；旧池等在用连接归还后关闭，不阻塞新配置生效。
func replacePool(pool *pgxpool.Pool, key [32]byte) {
	previous := maintenancePool.pool
	maintenancePool.pool, maintenancePool.key = pool, key
	if previous != nil {
		go previous.Close()
	}
}

func ClosePool() {
	maintenancePool.Lock()
	pool := maintenancePool.pool
	maintenancePool.pool = nil
	maintenancePool.Unlock()
	if pool != nil {
		pool.Close()
	}
}
