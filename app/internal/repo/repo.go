package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool 全局连接池。与存量系统一致：单库直连，应用不管理迁移（迁移走 migrations/*.sql）。
var Pool *pgxpool.Pool

// Connect 建立 PostgreSQL 连接池（对齐存量 config/database.js 的行为）。
func Connect(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	cfg.MaxConns = 20
	cfg.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	Pool = pool
	return nil
}

// Close 关闭连接池。
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}
