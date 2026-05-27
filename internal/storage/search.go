package storage

import (
	"fmt"

	"github.com/clipboardsync/clipboardsync/pkg/models"
)

// Search 在文本条目中搜索关键词（LIKE 查询）
func (db *DB) Search(query string, limit int) ([]*models.ClipboardEntry, error) {
	sqlQuery := `SELECT id, content_type, content_hash, payload, source_device, timestamp, size
				 FROM clipboard_entries
				 WHERE content_type = 'text' AND payload LIKE ?
				 ORDER BY timestamp DESC LIMIT ?`

	rows, err := db.conn.Query(sqlQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search entries: %w", err)
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
