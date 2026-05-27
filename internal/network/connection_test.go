package network

import (
	"net"
	"sync"
	"testing"
	"time"
)

// mockConn 用于测试的模拟连接
type mockConn struct {
	net.Conn
	closed bool
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func TestConnectionManagerAdd(t *testing.T) {
	cm := NewConnectionManager()
	peer := &PeerInfo{
		DeviceID: "device-1",
		Address:  "192.168.1.100",
		Port:     9527,
		Conn:     &mockConn{},
	}

	cm.Add(peer)

	got, exists := cm.Get("device-1")
	if !exists {
		t.Fatal("Get() 返回不存在")
	}
	if got.DeviceID != "device-1" {
		t.Errorf("DeviceID 不匹配: 期望 device-1, 实际 %s", got.DeviceID)
	}
}

func TestConnectionManagerRemove(t *testing.T) {
	cm := NewConnectionManager()
	conn := &mockConn{}
	peer := &PeerInfo{
		DeviceID: "device-1",
		Address:  "192.168.1.100",
		Port:     9527,
		Conn:     conn,
	}

	cm.Add(peer)
	cm.Remove("device-1")

	_, exists := cm.Get("device-1")
	if exists {
		t.Error("Remove 后 Get 仍返回存在")
	}
	if !conn.closed {
		t.Error("Remove 未关闭连接")
	}
}

func TestConnectionManagerGet(t *testing.T) {
	cm := NewConnectionManager()

	// 不存在的情况
	_, exists := cm.Get("nonexistent")
	if exists {
		t.Error("不存在的设备应返回 false")
	}

	// 存在的情况
	peer := &PeerInfo{
		DeviceID: "device-1",
		Address:  "192.168.1.100",
		Port:     9527,
	}
	cm.Add(peer)

	got, exists := cm.Get("device-1")
	if !exists {
		t.Fatal("Get() 返回不存在")
	}
	if got.Address != "192.168.1.100" {
		t.Errorf("Address 不匹配: 期望 192.168.1.100, 实际 %s", got.Address)
	}
}

func TestConnectionManagerList(t *testing.T) {
	cm := NewConnectionManager()

	// 空列表
	peers := cm.List()
	if len(peers) != 0 {
		t.Errorf("空管理器应返回空列表，实际 %d 个", len(peers))
	}

	// 添加多个对端
	for i := 0; i < 3; i++ {
		cm.Add(&PeerInfo{
			DeviceID: "device-" + string(rune('A'+i)),
			Address:  "192.168.1.100",
			Port:     9527 + i,
		})
	}

	peers = cm.List()
	if len(peers) != 3 {
		t.Errorf("期望 3 个对端，实际 %d 个", len(peers))
	}
}

func TestConnectionManagerCallbacks(t *testing.T) {
	cm := NewConnectionManager()

	var connectCalled, disconnectCalled bool
	var mu sync.Mutex

	cm.SetOnConnect(func(p *PeerInfo) {
		mu.Lock()
		connectCalled = true
		mu.Unlock()
	})

	cm.SetOnDisconnect(func(p *PeerInfo) {
		mu.Lock()
		disconnectCalled = true
		mu.Unlock()
	})

	peer := &PeerInfo{
		DeviceID: "device-1",
		Address:  "192.168.1.100",
		Port:     9527,
		Conn:     &mockConn{},
	}

	cm.Add(peer)

	// 等待回调执行
	// 注意：回调是异步执行的，这里简单等待
	// 在实际应用中可能需要更精确的同步机制
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !connectCalled {
		t.Error("OnConnect 回调未被调用")
	}
	mu.Unlock()

	cm.Remove("device-1")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !disconnectCalled {
		t.Error("OnDisconnect 回调未被调用")
	}
	mu.Unlock()
}
