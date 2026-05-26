package storage

import (
	"fmt"
	"testing"

	"clipboardsync/pkg/models"
)

func TestSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	entries := []*models.ClipboardEntry{
		{ID: "1", ContentType: "text", ContentHash: "h1", Payload: []byte("hello world"), SourceDevice: "d1", Timestamp: 1000, Size: 11},
		{ID: "2", ContentType: "text", ContentHash: "h2", Payload: []byte("foo bar"), SourceDevice: "d1", Timestamp: 2000, Size: 7},
		{ID: "3", ContentType: "text", ContentHash: "h3", Payload: []byte("hello again"), SourceDevice: "d1", Timestamp: 3000, Size: 11},
		{ID: "4", ContentType: "image", ContentHash: "h4", Payload: []byte("image data"), SourceDevice: "d1", Timestamp: 4000, Size: 10},
	}

	for _, e := range entries {
		if err := db.Store(e); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// 搜索 "hello"
	results, err := db.Search("hello", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// 验证只返回 text 类型
	for _, r := range results {
		if r.ContentType != "text" {
			t.Errorf("expected text type, got %s", r.ContentType)
		}
	}

	// 搜索不存在的关键词
	results, err = db.Search("nonexistent", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		entry := &models.ClipboardEntry{
			ID:           fmt.Sprintf("entry-%d", i),
			ContentType:  "text",
			ContentHash:  fmt.Sprintf("hash-%d", i),
			Payload:      []byte("test content"),
			SourceDevice: "device-1",
			Timestamp:    int64(i),
			Size:         12,
		}
		if err := db.Store(entry); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	results, err := db.Search("test", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}
