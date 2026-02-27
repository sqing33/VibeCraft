package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// Open 功能：打开 SQLite state DB，并设置 WAL/busy_timeout/foreign_keys 等运行参数。
// 参数/返回：ctx 用于控制初始化超时；dbPath 为 state.db 路径；返回 Store 与错误信息。
// 失败场景：driver 打开失败、Ping 失败或 pragma 设置失败时返回 error。
// 副作用：创建/打开 SQLite 文件；设置全局连接池为单连接；修改 DB pragma。
func Open(ctx context.Context, dbPath string) (*Store, error) {
	// modernc SQLite DSN: https://pkg.go.dev/modernc.org/sqlite
	// 这里保持 DSN 简单，关键 pragma 通过 Exec 设置，避免不同 driver 的参数差异。
	dsn := fmt.Sprintf("file:%s?mode=rwc", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// MVP：避免并发写导致 `database is locked`。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close 功能：关闭底层 DB 连接池。
// 参数/返回：无入参；成功返回 nil。
// 失败场景：关闭失败时返回 error。
// 副作用：释放 sqlite 文件句柄。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB 功能：暴露底层 sql.DB（用于更高层 service 调用或测试）。
// 参数/返回：无入参；返回 *sql.DB（可能为 nil）。
// 失败场景：无。
// 副作用：无。
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	// 注意：busy_timeout 是 per-connection 设置；我们强制单连接池，因此足够。
	stmts := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply pragma %q: %w", stmt, err)
		}
	}
	return nil
}
