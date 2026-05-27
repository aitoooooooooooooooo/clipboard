package network

import (
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestTLSConnectionAndMessageExchange 测试两个实例间的 TLS 连接和消息交换
// 注意：mDNS 发现在 localhost 上受限，此测试使用直接连接验证 TLS 传输层
func TestTLSConnectionAndMessageExchange(t *testing.T) {
	// 创建两个独立的证书目录（模拟两个设备）
	tmpDir := t.TempDir()
	certDirA := filepath.Join(tmpDir, "deviceA")
	certDirB := filepath.Join(tmpDir, "deviceB")

	// 生成两套证书
	certA, err := LoadOrCreateCert(certDirA)
	if err != nil {
		t.Fatalf("设备 A 证书生成失败: %v", err)
	}

	certB, err := LoadOrCreateCert(certDirB)
	if err != nil {
		t.Fatalf("设备 B 证书生成失败: %v", err)
	}

	// 设备 A：启动 TLS 服务器
	configA := TLSConfig(certA)
	var receivedMsg *Message
	var msgMu sync.Mutex
	msgReceived := make(chan struct{})

	serverA := NewTLSServer(configA, func(conn net.Conn) {
		defer conn.Close()
		msg, err := ReadMessage(conn)
		if err != nil {
			t.Errorf("设备 A 读取消息失败: %v", err)
			return
		}

		msgMu.Lock()
		receivedMsg = msg
		msgMu.Unlock()

		// 回复 Pong
		pong := &Message{Type: MsgPong, Payload: []byte("pong")}
		if err := WriteMessage(conn, pong); err != nil {
			t.Errorf("设备 A 发送 Pong 失败: %v", err)
		}

		close(msgReceived)
	})

	// 使用随机端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	if err := serverA.Start(port); err != nil {
		t.Fatalf("启动 TLS 服务器失败: %v", err)
	}
	defer serverA.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 设备 B：连接到设备 A
	configB := TLSConfig(certB)
	connB, err := TLSClient("127.0.0.1:"+itoa(port), configB)
	if err != nil {
		t.Fatalf("设备 B 连接失败: %v", err)
	}
	defer connB.Close()

	// 设备 B 发送 Ping
	ping := &Message{Type: MsgPing, Payload: []byte("ping")}
	if err := WriteMessage(connB, ping); err != nil {
		t.Fatalf("设备 B 发送 Ping 失败: %v", err)
	}

	// 等待设备 A 收到消息
	select {
	case <-msgReceived:
		// 消息已收到
	case <-time.After(5 * time.Second):
		t.Fatal("等待消息超时")
	}

	// 验证设备 A 收到的消息
	msgMu.Lock()
	if receivedMsg == nil {
		t.Fatal("设备 A 未收到消息")
	}
	if receivedMsg.Type != MsgPing {
		t.Errorf("消息类型错误: 期望 MsgPing(%d), 实际 %d", MsgPing, receivedMsg.Type)
	}
	if string(receivedMsg.Payload) != "ping" {
		t.Errorf("载荷错误: 期望 'ping', 实际 '%s'", string(receivedMsg.Payload))
	}
	msgMu.Unlock()

	// 设备 B 读取 Pong 回复
	pong, err := ReadMessage(connB)
	if err != nil {
		t.Fatalf("设备 B 读取 Pong 失败: %v", err)
	}
	if pong.Type != MsgPong {
		t.Errorf("Pong 类型错误: 期望 MsgPong(%d), 实际 %d", MsgPong, pong.Type)
	}
	if string(pong.Payload) != "pong" {
		t.Errorf("Pong 载荷错误: 期望 'pong', 实际 '%s'", string(pong.Payload))
	}

	t.Log("TLS 连接和消息交换测试通过")
}

// TestMultipleClientsConcurrent 测试多个客户端并发连接
func TestMultipleClientsConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "server")

	cert, err := LoadOrCreateCert(certDir)
	if err != nil {
		t.Fatalf("服务器证书生成失败: %v", err)
	}

	var connected sync.WaitGroup
	var received int
	var mu sync.Mutex

	config := TLSConfig(cert)
	server := NewTLSServer(config, func(conn net.Conn) {
		defer conn.Close()

		mu.Lock()
		received++
		mu.Unlock()

		connected.Done()

		// 保持连接直到测试结束
		time.Sleep(500 * time.Millisecond)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	if err := server.Start(port); err != nil {
		t.Fatalf("启动 TLS 服务器失败: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// 并发连接 5 个客户端
	numClients := 5
	connected.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			clientCert, _ := LoadOrCreateCert(filepath.Join(tmpDir, "client"))
			clientConfig := TLSConfig(clientCert)
			conn, err := TLSClient("127.0.0.1:"+itoa(port), clientConfig)
			if err != nil {
				t.Errorf("客户端连接失败: %v", err)
				return
			}
			defer conn.Close()
		}()
	}

	// 等待所有连接完成
	done := make(chan struct{})
	go func() {
		connected.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有连接已建立
	case <-time.After(5 * time.Second):
		t.Fatal("等待并发连接超时")
	}

	mu.Lock()
	if received != numClients {
		t.Errorf("连接数不匹配: 期望 %d, 实际 %d", numClients, received)
	}
	mu.Unlock()

	t.Log("并发连接测试通过")
}

// itoa 整数转字符串
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
