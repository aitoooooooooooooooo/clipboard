package network

import (
	"bytes"
	"testing"
)

func TestMessageEncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		payload []byte
	}{
		{
			name:    "SyncEntry with JSON payload",
			msgType: MsgSyncEntry,
			payload: []byte(`{"id":"123","content":"hello"}`),
		},
		{
			name:    "AckEntry empty payload",
			msgType: MsgAckEntry,
			payload: []byte{},
		},
		{
			name:    "Ping heartbeat",
			msgType: MsgPing,
			payload: nil,
		},
		{
			name:    "Pong heartbeat response",
			msgType: MsgPong,
			payload: nil,
		},
		{
			name:    "RequestHistory",
			msgType: MsgRequestHistory,
			payload: []byte(`{"since":"2024-01-01"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Type:    tt.msgType,
				Payload: tt.payload,
			}

			// 编码
			data, err := msg.Encode()
			if err != nil {
				t.Fatalf("Encode() 失败: %v", err)
			}

			// 验证长度字段
			if len(data) < 4 {
				t.Fatalf("编码数据过短: %d", len(data))
			}

			// 解码
			decoded, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode() 失败: %v", err)
			}

			// 验证类型
			if decoded.Type != msg.Type {
				t.Errorf("消息类型不匹配: 期望 %d, 实际 %d", msg.Type, decoded.Type)
			}

			// 验证载荷
			if !bytes.Equal(decoded.Payload, msg.Payload) {
				t.Errorf("载荷不匹配: 期望 %q, 实际 %q", msg.Payload, decoded.Payload)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "数据过短",
			data: []byte{0x00, 0x01},
		},
		{
			name: "空数据",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.data)
			if err == nil {
				t.Error("期望解码错误，但未返回")
			}
		})
	}
}

func TestReadWriteMessage(t *testing.T) {
	msg := &Message{
		Type:    MsgSyncEntry,
		Payload: []byte(`{"test": true}`),
	}

	// 使用 buffer 模拟网络连接
	var buf bytes.Buffer

	// 写入消息
	if err := WriteMessage(&buf, msg); err != nil {
		t.Fatalf("WriteMessage() 失败: %v", err)
	}

	// 读取消息
	decoded, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage() 失败: %v", err)
	}

	// 验证
	if decoded.Type != msg.Type {
		t.Errorf("类型不匹配: 期望 %d, 实际 %d", msg.Type, decoded.Type)
	}
	if !bytes.Equal(decoded.Payload, msg.Payload) {
		t.Errorf("载荷不匹配: 期望 %q, 实际 %q", msg.Payload, decoded.Payload)
	}
}

func TestMessageSizeLimit(t *testing.T) {
	// 测试接近上限的消息
	largePayload := make([]byte, MaxMessageSize-10)
	msg := &Message{
		Type:    MsgSyncEntry,
		Payload: largePayload,
	}

	_, err := msg.Encode()
	if err != nil {
		t.Fatalf("编码大消息失败: %v", err)
	}

	// 测试超大消息
	tooLargePayload := make([]byte, MaxMessageSize)
	msg2 := &Message{
		Type:    MsgSyncEntry,
		Payload: tooLargePayload,
	}

	_, err = msg2.Encode()
	if err == nil {
		t.Error("期望超大消息编码失败，但未返回错误")
	}
}
