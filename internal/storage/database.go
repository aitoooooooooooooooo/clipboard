package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// DB 封装数据库连接和操作
type DB struct {
	conn *sql.DB
}

// Open 打开或创建 SQLite 数据库
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	conn.SetMaxOpenConns(1) // SQLite 单写入者
	conn.SetMaxIdleConns(1)

	// 启用 WAL 模式和外键约束
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Migrate 创建表结构和索引
func (db *DB) Migrate() error {
	migrations := []string{
		// clipboard_entries 表
		`CREATE TABLE IF NOT EXISTS clipboard_entries (
			id TEXT PRIMARY KEY,
			content_type TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			payload BLOB,
			source_device TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			size INTEGER NOT NULL
		)`,
		// content_hash 索引（用于去重查询）
		`CREATE INDEX IF NOT EXISTS idx_clipboard_entries_content_hash ON clipboard_entries(content_hash)`,
		// timestamp 索引（用于排序和 LRU 淘汰）
		`CREATE INDEX IF NOT EXISTS idx_clipboard_entries_timestamp ON clipboard_entries(timestamp)`,

		// devices 表
		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			paired BOOLEAN NOT NULL DEFAULT 0,
			last_seen INTEGER NOT NULL,
			public_key BLOB
		)`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.conn.Close()
}
