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

// HistoryRequestPayload 历史请求载荷
type HistoryRequestPayload struct {
	Limit int `json:"limit"` // 请求的条目数量上限
}

// PairingRequestPayload 配对请求载荷
type PairingRequestPayload struct {
	Code      string `json:"code"`       // 6 位配对码
	DeviceID  string `json:"device_id"`  // 请求方设备 ID
	PublicKey []byte `json:"public_key"` // 请求方公钥
}

// PairingResponsePayload 配对响应载荷
type PairingResponsePayload struct {
	Accepted bool   `json:"accepted"` // 是否接受配对
	DeviceID string `json:"device_id"` // 响应方设备 ID
	Error    string `json:"error"`     // 错误信息（拒绝时）
}

// SyncManager 管理剪贴板同步逻辑
type SyncManager struct {
	connMgr         *ConnectionManager
	storage         *storage.DB
	deviceID        string
	onNewEntry      func(*models.ClipboardEntry)
	onPairingReq    func(*PairingRequestPayload) *PairingResponsePayload
	onPairingResp   func(*PairingResponsePayload)
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

// SetOnPairingRequest 设置收到配对请求时的回调
func (sm *SyncManager) SetOnPairingRequest(handler func(*PairingRequestPayload) *PairingResponsePayload) {
	sm.onPairingReq = handler
}

// SetOnPairingResponse 设置收到配对响应时的回调
func (sm *SyncManager) SetOnPairingResponse(handler func(*PairingResponsePayload)) {
	sm.onPairingResp = handler
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
	case MsgRequestHistory:
		return sm.handleHistoryRequest(from, msg.Payload)
	case MsgPairingRequest:
		return sm.handlePairingRequest(from, msg.Payload)
	case MsgPairingAccept:
		return sm.handlePairingAccept(from, msg.Payload)
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

// RequestHistory 向指定对端请求历史条目
func (sm *SyncManager) RequestHistory(peerID string) error {
	reqPayload := HistoryRequestPayload{Limit: 100}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("序列化历史请求失败: %w", err)
	}

	msg := &Message{
		Type:    MsgRequestHistory,
		Payload: payloadBytes,
	}

	peer, exists := sm.connMgr.Get(peerID)
	if !exists {
		return fmt.Errorf("对端未连接: %s", peerID)
	}

	if err := WriteMessage(peer.Conn, msg); err != nil {
		return fmt.Errorf("发送历史请求失败: %w", err)
	}

	slog.Debug("已发送历史请求", "peer", peerID)
	return nil
}

// handleHistoryRequest 处理收到的历史请求：将本机条目发送给请求方
func (sm *SyncManager) handleHistoryRequest(from string, payloadBytes []byte) error {
	var payload HistoryRequestPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("反序列化历史请求失败: %w", err)
	}

	limit := payload.Limit
	if limit <= 0 {
		limit = 100
	}

	// 从数据库读取最近的条目
	entries, err := sm.storage.List(limit)
	if err != nil {
		return fmt.Errorf("读取历史条目失败: %w", err)
	}

	// 获取对端连接
	peer, exists := sm.connMgr.Get(from)
	if !exists {
		slog.Warn("发送历史失败：对端已断开", "device_id", from)
		return nil
	}

	// 逐条发送
	sent := 0
	for _, entry := range entries {
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
			slog.Warn("序列化条目失败", "entry_id", entry.ID, "error", err)
			continue
		}

		msg := &Message{
			Type:    MsgSyncEntry,
			Payload: payloadBytes,
		}

		if err := WriteMessage(peer.Conn, msg); err != nil {
			slog.Warn("发送历史条目失败", "entry_id", entry.ID, "error", err)
			break
		}
		sent++
	}

	slog.Info("历史条目已发送", "to", from, "count", sent)
	return nil
}

// SendPairingRequest 向所有已连接对端发送配对请求
func (sm *SyncManager) SendPairingRequest(code string, deviceID string, publicKey []byte) error {
	reqPayload := PairingRequestPayload{
		Code:      code,
		DeviceID:  deviceID,
		PublicKey: publicKey,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("序列化配对请求失败: %w", err)
	}

	msg := &Message{
		Type:    MsgPairingRequest,
		Payload: payloadBytes,
	}

	if err := sm.connMgr.Broadcast(msg); err != nil {
		return fmt.Errorf("广播配对请求失败: %w", err)
	}

	slog.Info("配对请求已发送", "code", code)
	return nil
}

// SendPairingResponse 向指定对端发送配对响应
func (sm *SyncManager) SendPairingResponse(peerID string, resp *PairingResponsePayload) error {
	payloadBytes, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("序列化配对响应失败: %w", err)
	}

	msg := &Message{
		Type:    MsgPairingAccept,
		Payload: payloadBytes,
	}

	peer, exists := sm.connMgr.Get(peerID)
	if !exists {
		return fmt.Errorf("对端未连接: %s", peerID)
	}

	if err := WriteMessage(peer.Conn, msg); err != nil {
		return fmt.Errorf("发送配对响应失败: %w", err)
	}

	slog.Info("配对响应已发送", "to", peerID, "accepted", resp.Accepted)
	return nil
}

// handlePairingRequest 处理收到的配对请求
func (sm *SyncManager) handlePairingRequest(from string, payloadBytes []byte) error {
	var payload PairingRequestPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("反序列化配对请求失败: %w", err)
	}

	slog.Info("收到配对请求", "from", from, "code", payload.Code, "device_id", payload.DeviceID)

	// 调用回调处理配对逻辑
	if sm.onPairingReq == nil {
		// 没有处理器，拒绝配对
		resp := &PairingResponsePayload{Accepted: false, DeviceID: sm.deviceID, Error: "配对未启用"}
		return sm.SendPairingResponse(from, resp)
	}

	resp := sm.onPairingReq(&payload)
	return sm.SendPairingResponse(from, resp)
}

// handlePairingAccept 处理收到的配对响应
func (sm *SyncManager) handlePairingAccept(from string, payloadBytes []byte) error {
	var payload PairingResponsePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("反序列化配对响应失败: %w", err)
	}

	if payload.Accepted {
		slog.Info("配对已被接受", "peer", from, "device_id", payload.DeviceID)
	} else {
		slog.Info("配对被拒绝", "peer", from, "reason", payload.Error)
	}

	// 触发回调
	if sm.onPairingResp != nil {
		sm.onPairingResp(&payload)
	}

	return nil
}
