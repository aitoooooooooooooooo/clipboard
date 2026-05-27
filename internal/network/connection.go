package network

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// PeerInfo 对端信息
type PeerInfo struct {
	DeviceID    string
	DisplayName string
	Address     string
	Port        int
	Conn        net.Conn
}

// ConnectionManager 管理所有已连接的对端
type ConnectionManager struct {
	peers         map[string]*PeerInfo
	mu            sync.RWMutex
	onConnect     func(*PeerInfo)
	onDisconnect  func(*PeerInfo)
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		peers: make(map[string]*PeerInfo),
	}
}

// Add 添加一个已连接的对端
func (cm *ConnectionManager) Add(peer *PeerInfo) {
	cm.mu.Lock()
	cm.peers[peer.DeviceID] = peer
	cm.mu.Unlock()

	slog.Info("对端已连接", "device_id", peer.DeviceID, "addr", peer.Address)

	if cm.onConnect != nil {
		go cm.onConnect(peer)
	}
}

// Remove 移除对端连接
func (cm *ConnectionManager) Remove(deviceID string) {
	cm.mu.Lock()
	peer, exists := cm.peers[deviceID]
	if exists {
		delete(cm.peers, deviceID)
		if peer.Conn != nil {
			peer.Conn.Close()
		}
	}
	cm.mu.Unlock()

	if exists {
		slog.Info("对端已断开", "device_id", deviceID)
		if cm.onDisconnect != nil {
			go cm.onDisconnect(peer)
		}
	}
}

// Get 根据设备 ID 获取连接信息
func (cm *ConnectionManager) Get(deviceID string) (*PeerInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	peer, exists := cm.peers[deviceID]
	return peer, exists
}

// List 获取所有已连接的对端
func (cm *ConnectionManager) List() []*PeerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(cm.peers))
	for _, p := range cm.peers {
		peers = append(peers, p)
	}
	return peers
}

// Broadcast 向所有已连接的对端广播消息
func (cm *ConnectionManager) Broadcast(msg *Message) error {
	cm.mu.RLock()
	peers := make([]*PeerInfo, 0, len(cm.peers))
	for _, p := range cm.peers {
		peers = append(peers, p)
	}
	cm.mu.RUnlock()

	var firstErr error
	for _, peer := range peers {
		if err := WriteMessage(peer.Conn, msg); err != nil {
			slog.Error("广播消息失败", "device_id", peer.DeviceID, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("广播到 %s 失败: %w", peer.DeviceID, err)
			}
			// 移除失败的连接
			cm.Remove(peer.DeviceID)
		}
	}

	return firstErr
}

// SetOnConnect 设置新连接回调
func (cm *ConnectionManager) SetOnConnect(handler func(*PeerInfo)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onConnect = handler
}

// SetOnDisconnect 设置断开连接回调
func (cm *ConnectionManager) SetOnDisconnect(handler func(*PeerInfo)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onDisconnect = handler
}

// Close 关闭所有连接
func (cm *ConnectionManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, peer := range cm.peers {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
		delete(cm.peers, id)
	}
}
