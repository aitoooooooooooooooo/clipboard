package network

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/clipboardsync/clipboardsync/internal/storage"
	"github.com/clipboardsync/clipboardsync/pkg/models"
)

// SyncEntryPayload 同步条目消息载荷
type SyncEntryPayload struct {
	ID           string `json:"id"`
	ContentType  string `json:"content_type"`
	ContentHash  string `json:"content_hash"`
	Payload      []byte `json:"payload"`
	SourceDevice string `json:"source_device"`
	Timestamp    int64  `json:"timestamp"`
	Size         int64  `json:"size"`
}

// AckEntryPayload 确认消息载荷
type AckEntryPayload struct {
	EntryID string `json:"entry_id"`
	Status  string `json:"status"` // "ok" 或 "duplicate"
}

// SyncManager 管理剪贴板同步逻辑
type SyncManager struct {
	connMgr    *ConnectionManager
	storage    *storage.DB
	deviceID   string
	onNewEntry func(*models.ClipboardEntry)
}

// NewSyncManager 创建同步管理器
func NewSyncManager(connMgr *ConnectionManager, storage *storage.DB, deviceID string) *SyncManager {
	return &SyncManager{
		connMgr:  connMgr,
		storage:  storage,
		deviceID: deviceID,
	}
}

// SetOnNewEntry 设置收到新条目时的回调
func (sm *SyncManager) SetOnNewEntry(handler func(*models.ClipboardEntry)) {
	sm.onNewEntry = handler
}

// BroadcastEntry 将新条目广播给所有已连接的对端
func (sm *SyncManager) BroadcastEntry(entry *models.ClipboardEntry) error {
	payload := SyncEntryPayload{
		ID:           entry.ID,
		ContentType:  entry.ContentType,
		ContentHash:  entry.ContentHash,
		Payload:      entry.Payload,
		SourceDevice: entry.SourceDevice,
		Timestamp:    entry.Timestamp,
		Size:         entry.Size,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化同步载荷失败: %w", err)
	}

	msg := &Message{
		Type:    MsgSyncEntry,
		Payload: payloadBytes,
	}

	if err := sm.connMgr.Broadcast(msg); err != nil {
		return fmt.Errorf("广播同步条目失败: %w", err)
	}

	slog.Debug("同步条目已广播", "entry_id", entry.ID, "hash", entry.ContentHash)
	return nil
}

// HandleMessage 处理收到的同步消息
func (sm *SyncManager) HandleMessage(from string, msg *Message) error {
	switch msg.Type {
	case MsgSyncEntry:
		return sm.handleSyncEntry(from, msg.Payload)
	case MsgAckEntry:
		return sm.handleAckEntry(from, msg.Payload)
	default:
		return fmt.Errorf("未知消息类型: %d", msg.Type)
	}
}

// handleSyncEntry 处理收到的同步条目
func (sm *SyncManager) handleSyncEntry(from string, payloadBytes []byte) error {
	var payload SyncEntryPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("反序列化同步载荷失败: %w", err)
	}

	// 检查是否是自己发出的（避免回环）
	if payload.SourceDevice == sm.deviceID {
		return nil
	}

	// 检查内容哈希是否已存在
	exists, err := sm.storage.ExistsByHash(payload.ContentHash)
	if err != nil {
		return fmt.Errorf("检查哈希存在性失败: %w", err)
	}

	ackStatus := "ok"
	if exists {
		ackStatus = "duplicate"
		slog.Debug("重复内容已跳过", "hash", payload.ContentHash, "from", from)
	} else {
		// 存储新条目
		entry := &models.ClipboardEntry{
			ID:           payload.ID,
			ContentType:  payload.ContentType,
			ContentHash:  payload.ContentHash,
			Payload:      payload.Payload,
			SourceDevice: payload.SourceDevice,
			Timestamp:    payload.Timestamp,
			Size:         payload.Size,
		}

		if err := sm.storage.Store(entry); err != nil {
			return fmt.Errorf("存储同步条目失败: %w", err)
		}

		slog.Info("收到新同步条目", "entry_id", payload.ID, "from", from)

		// 触发回调
		if sm.onNewEntry != nil {
			go sm.onNewEntry(entry)
		}
	}

	// 发送确认
	ack := AckEntryPayload{
		EntryID: payload.ID,
		Status:  ackStatus,
	}

	ackBytes, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("序列化确认载荷失败: %w", err)
	}

	ackMsg := &Message{
		Type:    MsgAckEntry,
		Payload: ackBytes,
	}

	// 发送给发送方
	peer, exists := sm.connMgr.Get(from)
	if !exists {
		slog.Warn("发送确认失败：对端已断开", "device_id", from)
		return nil
	}

	if err := WriteMessage(peer.Conn, ackMsg); err != nil {
		return fmt.Errorf("发送确认失败: %w", err)
	}

	return nil
}

// handleAckEntry 处理收到的确认消息
func (sm *SyncManager) handleAckEntry(from string, payloadBytes []byte) error {
	var payload AckEntryPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("反序列化确认载荷失败: %w", err)
	}

	slog.Debug("收到确认", "entry_id", payload.EntryID, "status", payload.Status, "from", from)
	return nil
}
