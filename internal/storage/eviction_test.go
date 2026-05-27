package storage

import (
	"fmt"
	"testing"

	"github.com/clipboardsync/clipboardsync/pkg/models"
)

func TestEvict(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 插入 1005 条记录
	for i := 0; i < 1005; i++ {
		entry := &models.ClipboardEntry{
			ID:           fmt.Sprintf("entry-%d", i),
			ContentType:  "text",
			ContentHash:  fmt.Sprintf("hash-%d", i),
			Payload:      []byte(fmt.Sprintf("content %d", i)),
			SourceDevice: "device-1",
			Timestamp:    int64(i),
			Size:         10,
		}
		if err := db.Store(entry); err != nil {
			t.Fatalf("Store failed at %d: %v", i, err)
		}
	}

	// 验证条目数不超过 1000
	entries, err := db.List(2000)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) > 1000 {
		t.Errorf("expected at most 1000 entries, got %d", len(entries))
	}

	// 验证最新的条目被保留
	found := false
	for _, e := range entries {
		if e.ID == "entry-1004" {
			found = true
			break
		}
	}
	if !found {
		t.Error("newest entry not found after eviction")
	}
}
