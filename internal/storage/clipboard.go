package storage

import (
	"fmt"

	"clipboardsync/pkg/models"

	"github.com/mattn/go-sqlite3"
)

// Store 存储剪贴板条目（插入前检查哈希去重）
func (db *DB) Store(entry *models.ClipboardEntry) error {
	// 检查哈希是否已存在
	exists, err := db.ExistsByHash(entry.ContentHash)
	if err != nil {
		return fmt.Errorf("failed to check hash existence: %w", err)
	}
	if exists {
		return fmt.Errorf("duplicate content hash: %s", entry.ContentHash)
	}

	query := `INSERT INTO clipboard_entries (id, content_type, content_hash, payload, source_device, timestamp, size)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = db.conn.Exec(query,
		entry.ID,
		entry.ContentType,
		entry.ContentHash,
		entry.Payload,
		entry.SourceDevice,
		entry.Timestamp,
		entry.Size,
	)
	if err != nil {
		// 检查是否是唯一约束冲突（并发场景）
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.Code == sqlite3.ErrConstraint {
			return fmt.Errorf("duplicate content hash: %s", entry.ContentHash)
		}
		return fmt.Errorf("failed to store entry: %w", err)
	}

	// 插入成功后执行 LRU 淘汰
	if err := db.Evict(1000); err != nil {
		return fmt.Errorf("failed to evict old entries: %w", err)
	}

	return nil
}

// Get 根据 ID 获取条目
func (db *DB) Get(id string) (*models.ClipboardEntry, error) {
	query := `SELECT id, content_type, content_hash, payload, source_device, timestamp, size
			  FROM clipboard_entries WHERE id = ?`
	entry := &models.ClipboardEntry{}
	err := db.conn.QueryRow(query, id).Scan(
		&entry.ID,
		&entry.ContentType,
		&entry.ContentHash,
		&entry.Payload,
		&entry.SourceDevice,
		&entry.Timestamp,
		&entry.Size,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}
	return entry, nil
}

// List 获取最近 N 条记录（按时间倒序）
func (db *DB) List(limit int) ([]*models.ClipboardEntry, error) {
	query := `SELECT id, content_type, content_hash, payload, source_device, timestamp, size
			  FROM clipboard_entries ORDER BY timestamp DESC LIMIT ?`
	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.ClipboardEntry
	for rows.Next() {
		entry := &models.ClipboardEntry{}
		if err := rows.Scan(
			&entry.ID,
			&entry.ContentType,
			&entry.ContentHash,
			&entry.Payload,
			&entry.SourceDevice,
			&entry.Timestamp,
			&entry.Size,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Delete 根据 ID 删除条目
func (db *DB) Delete(id string) error {
	query := `DELETE FROM clipboard_entries WHERE id = ?`
	result, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("entry not found: %s", id)
	}
	return nil
}

// ExistsByHash 检查哈希是否已存在（去重用）
func (db *DB) ExistsByHash(hash string) (bool, error) {
	query := `SELECT COUNT(*) FROM clipboard_entries WHERE content_hash = ?`
	var count int
	err := db.conn.QueryRow(query, hash).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check hash existence: %w", err)
	}
	return count > 0, nil
}
