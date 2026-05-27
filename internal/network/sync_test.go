package network

import (
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/clipboardsync/clipboardsync/internal/storage"
	"github.com/clipboardsync/clipboardsync/pkg/models"
)

// mockConn 用于测试的模拟连接
type syncMockConn struct {
	net.Conn
	written []byte
}

func (m *syncMockConn) Write(b []byte) (int, error) {
	m.written = append(m.written, b...)
	return len(b), nil
}

func (m *syncMockConn) Close() error {
	return nil
}

func setupTestDB(t *testing.T) *storage.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "sync-test-*.db")
	if err != nil {
		t.Fatalf("创建临时数据库失败: %v", err)
	}
	tmpFile.Close()

	db, err := storage.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpFile.Name())
	})
	return db
}

func TestBroadcastEntry(t *testing.T) {
	connMgr := NewConnectionManager()
	db := setupTestDB(t)
	sm := NewSyncManager(connMgr, db, "device-A")

	// 添加模拟对端
	conn1 := &syncMockConn{}
	conn2 := &syncMockConn{}
	connMgr.Add(&PeerInfo{DeviceID: "device-B", Conn: conn1})
	connMgr.Add(&PeerInfo{DeviceID: "device-C", Conn: conn2})

	// 创建测试条目
	entry := &models.ClipboardEntry{
		ID:           "entry-1",
		ContentType:  "text/plain",
		ContentHash:  "abc123",
		Payload:      []byte("hello"),
		SourceDevice: "device-A",
		Timestamp:    time.Now().UnixMilli(),
		Size:         5,
	}

	// 广播
	if err := sm.BroadcastEntry(entry); err != nil {
		t.Fatalf("BroadcastEntry 失败: %v", err)
	}

	// 验证两个对端都收到了消息
	if len(conn1.written) == 0 {
		t.Error("device-B 未收到消息")
	}
	if len(conn2.written) == 0 {
		t.Error("device-C 未收到消息")
	}
}

func TestHandleMessageNewEntry(t *testing.T) {
	connMgr := NewConnectionManager()
	db := setupTestDB(t)
	sm := NewSyncManager(connMgr, db, "device-A")

	// 添加发送方对端
	conn := &syncMockConn{}
	connMgr.Add(&PeerInfo{DeviceID: "device-B", Conn: conn})

	// 设置回调
	var receivedEntry *models.ClipboardEntry
	sm.SetOnNewEntry(func(entry *models.ClipboardEntry) {
		receivedEntry = entry
	})

	// 创建同步消息
	payload := SyncEntryPayload{
		ID:           "entry-1",
		ContentType:  "text/plain",
		ContentHash:  "hash-123",
		Payload:      []byte("test content"),
		SourceDevice: "device-B",
		Timestamp:    time.Now().UnixMilli(),
		Size:         12,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &Message{
		Type:    MsgSyncEntry,
		Payload: payloadBytes,
	}

	// 处理消息
	if err := sm.HandleMessage("device-B", msg); err != nil {
		t.Fatalf("HandleMessage 失败: %v", err)
	}

	// 等待异步回调
	time.Sleep(100 * time.Millisecond)

	// 验证条目已存储
	exists, err := db.ExistsByHash("hash-123")
	if err != nil {
		t.Fatalf("ExistsByHash 失败: %v", err)
	}
	if !exists {
		t.Error("条目未存储到数据库")
	}

	// 验证回调被调用
	if receivedEntry == nil {
		t.Error("OnNewEntry 回调未被调用")
	}

	// 验证发送了确认
	if len(conn.written) == 0 {
		t.Error("未发送确认消息")
	}
}

func TestHandleMessageDuplicateEntry(t *testing.T) {
	connMgr := NewConnectionManager()
	db := setupTestDB(t)
	sm := NewSyncManager(connMgr, db, "device-A")

	// 添加发送方对端
	conn := &syncMockConn{}
	connMgr.Add(&PeerInfo{DeviceID: "device-B", Conn: conn})

	// 先存储一个条目
	entry := &models.ClipboardEntry{
		ID:           "existing-entry",
		ContentType:  "text/plain",
		ContentHash:  "duplicate-hash",
		Payload:      []byte("existing content"),
		SourceDevice: "device-B",
		Timestamp:    time.Now().UnixMilli(),
		Size:         16,
	}
	if err := db.Store(entry); err != nil {
		t.Fatalf("预存条目失败: %v", err)
	}

	// 创建重复的同步消息
	payload := SyncEntryPayload{
		ID:           "new-entry",
		ContentType:  "text/plain",
		ContentHash:  "duplicate-hash",
		Payload:      []byte("new content"),
		SourceDevice: "device-B",
		Timestamp:    time.Now().UnixMilli(),
		Size:         11,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &Message{
		Type:    MsgSyncEntry,
		Payload: payloadBytes,
	}

	// 处理消息
	if err := sm.HandleMessage("device-B", msg); err != nil {
		t.Fatalf("HandleMessage 失败: %v", err)
	}

	// 验证发送了 duplicate 确认
	if len(conn.written) > 0 {
		// 解析确认消息
		ackMsg, err := Decode(conn.written)
		if err != nil {
			t.Fatalf("解码确认消息失败: %v", err)
		}
		if ackMsg.Type != MsgAckEntry {
			t.Errorf("确认消息类型错误: 期望 %d, 实际 %d", MsgAckEntry, ackMsg.Type)
		}
		var ackPayload AckEntryPayload
		if err := json.Unmarshal(ackMsg.Payload, &ackPayload); err != nil {
			t.Fatalf("反序列化确认载荷失败: %v", err)
		}
		if ackPayload.Status != "duplicate" {
			t.Errorf("确认状态错误: 期望 duplicate, 实际 %s", ackPayload.Status)
		}
	}
}

func TestHandleMessageIgnoreSelf(t *testing.T) {
	connMgr := NewConnectionManager()
	db := setupTestDB(t)
	sm := NewSyncManager(connMgr, db, "device-A")

	// 创建来自自己的同步消息
	payload := SyncEntryPayload{
		ID:           "entry-1",
		ContentType:  "text/plain",
		ContentHash:  "self-hash",
		Payload:      []byte("self content"),
		SourceDevice: "device-A", // 来自自己
		Timestamp:    time.Now().UnixMilli(),
		Size:         12,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &Message{
		Type:    MsgSyncEntry,
		Payload: payloadBytes,
	}

	// 处理消息
	if err := sm.HandleMessage("device-B", msg); err != nil {
		t.Fatalf("HandleMessage 失败: %v", err)
	}

	// 验证条目未存储（自己的消息应该被忽略）
	exists, err := db.ExistsByHash("self-hash")
	if err != nil {
		t.Fatalf("ExistsByHash 失败: %v", err)
	}
	if exists {
		t.Error("自己的消息不应该被存储")
	}
}
