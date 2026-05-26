package storage

import "fmt"

// Evict 执行 LRU 淘汰，确保条目数不超过 maxEntries
func (db *DB) Evict(maxEntries int) error {
	// 获取当前条目数
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM clipboard_entries").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count entries: %w", err)
	}

	// 如果超过限制，删除最旧的条目
	if count > maxEntries {
		toDelete := count - maxEntries
		query := `DELETE FROM clipboard_entries WHERE id IN (
			SELECT id FROM clipboard_entries ORDER BY timestamp ASC LIMIT ?
		)`
		_, err := db.conn.Exec(query, toDelete)
		if err != nil {
			return fmt.Errorf("failed to evict entries: %w", err)
		}
	}
	return nil
}
