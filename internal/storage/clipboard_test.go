package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clipboardsync/clipboardsync/pkg/models"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "clipboard-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Open failed: %v", err)
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("Migrate failed: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
	return db, cleanup
}

func TestStore(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	entry := &models.ClipboardEntry{
		ID:           "test-1",
		ContentType:  "text",
		ContentHash:  "hash-1",
		Payload:      []byte("hello world"),
		SourceDevice: "device-1",
		Timestamp:    1000,
		Size:         11,
	}

	if err := db.Store(entry); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// 验证可以获取
	got, err := db.Get("test-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != entry.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, entry.ID)
	}
}

func TestStoreDuplicateHash(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	entry1 := &models.ClipboardEntry{
		ID:           "test-1",
		ContentType:  "text",
		ContentHash:  "hash-1",
		Payload:      []byte("hello"),
		SourceDevice: "device-1",
		Timestamp:    1000,
		Size:         5,
	}

	entry2 := &models.ClipboardEntry{
		ID:           "test-2",
		ContentType:  "text",
		ContentHash:  "hash-1", // 相同哈希
		Payload:      []byte("hello"),
		SourceDevice: "device-2",
		Timestamp:    2000,
		Size:         5,
	}

	if err := db.Store(entry1); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// 尝试存储重复哈希，应该失败
	if err := db.Store(entry2); err == nil {
		t.Error("expected error for duplicate hash, got nil")
	}
}

func TestList(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	entries := []*models.ClipboardEntry{
		{ID: "1", ContentType: "text", ContentHash: "h1", Payload: []byte("a"), SourceDevice: "d1", Timestamp: 1000, Size: 1},
		{ID: "2", ContentType: "text", ContentHash: "h2", Payload: []byte("b"), SourceDevice: "d1", Timestamp: 2000, Size: 1},
		{ID: "3", ContentType: "text", ContentHash: "h3", Payload: []byte("c"), SourceDevice: "d1", Timestamp: 3000, Size: 1},
	}

	for _, e := range entries {
		if err := db.Store(e); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// 获取最近 2 条
	got, err := db.List(2)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	// 应该按时间倒序
	if got[0].ID != "3" {
		t.Errorf("expected ID 3, got %s", got[0].ID)
	}
	if got[1].ID != "2" {
		t.Errorf("expected ID 2, got %s", got[1].ID)
	}
}

func TestDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	entry := &models.ClipboardEntry{
		ID:           "test-1",
		ContentType:  "text",
		ContentHash:  "hash-1",
		Payload:      []byte("hello"),
		SourceDevice: "device-1",
		Timestamp:    1000,
		Size:         5,
	}

	if err := db.Store(entry); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if err := db.Delete("test-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证已删除
	_, err := db.Get("test-1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestExistsByHash(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	entry := &models.ClipboardEntry{
		ID:           "test-1",
		ContentType:  "text",
		ContentHash:  "hash-1",
		Payload:      []byte("hello"),
		SourceDevice: "device-1",
		Timestamp:    1000,
		Size:         5,
	}

	// 存储前检查
	exists, err := db.ExistsByHash("hash-1")
	if err != nil {
		t.Fatalf("ExistsByHash failed: %v", err)
	}
	if exists {
		t.Error("expected hash not to exist before store")
	}

	// 存储后检查
	if err := db.Store(entry); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	exists, err = db.ExistsByHash("hash-1")
	if err != nil {
		t.Fatalf("ExistsByHash failed: %v", err)
	}
	if !exists {
		t.Error("expected hash to exist after store")
	}
}
