package network

import (
	"encoding/binary"
	"fmt"
	"io"
)

// 消息长度上限：50MB + 4字节长度头 + 1字节类型头
const MaxMessageSize = 50*1024*1024 + 5

// MessageType 消息类型枚举
type MessageType byte

const (
	MsgSyncEntry      MessageType = 0x01 // 同步条目
	MsgAckEntry       MessageType = 0x02 // 确认收到
	MsgRequestHistory MessageType = 0x03 // 请求历史（MVP 可选）
	MsgPing           MessageType = 0x10 // 心跳
	MsgPong           MessageType = 0x11 // 心跳响应
)

// Message 网络消息
type Message struct {
	Type    MessageType
	Payload []byte
}

// Encode 将消息编码为字节流
// 格式：[4字节长度(大端序)] [1字节类型] [N字节载荷]
// 长度字段值 = 1 + len(Payload)，不含长度字段自身的4字节
func (m *Message) Encode() ([]byte, error) {
	payloadLen := len(m.Payload)
	if payloadLen+1 > MaxMessageSize {
		return nil, fmt.Errorf("消息载荷过大: %d 字节，上限 %d 字节", payloadLen, MaxMessageSize-5)
	}

	totalLen := 1 + payloadLen // 类型字节 + 载荷
	buf := make([]byte, 4+totalLen)

	binary.BigEndian.PutUint32(buf[:4], uint32(totalLen))
	buf[4] = byte(m.Type)
	copy(buf[5:], m.Payload)

	return buf, nil
}

// Decode 从字节流解码消息
// 输入应包含完整的 [长度][类型][载荷] 结构
func Decode(data []byte) (*Message, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("数据过短: 需要至少 4 字节，实际 %d 字节", len(data))
	}

	msgLen := binary.BigEndian.Uint32(data[:4])
	if msgLen > uint32(MaxMessageSize) {
		return nil, fmt.Errorf("消息长度超出上限: %d 字节", msgLen)
	}

	if uint32(len(data)) < 4+msgLen {
		return nil, fmt.Errorf("数据不完整: 需要 %d 字节，实际 %d 字节", 4+msgLen, len(data))
	}

	if msgLen < 1 {
		return nil, fmt.Errorf("消息长度无效: 至少需要 1 字节（类型），实际 %d 字节", msgLen)
	}

	msg := &Message{
		Type:    MessageType(data[4]),
		Payload: make([]byte, msgLen-1),
	}
	copy(msg.Payload, data[5:4+msgLen])

	return msg, nil
}

// ReadMessage 从连接中读取一条完整消息
func ReadMessage(r io.Reader) (*Message, error) {
	// 读取 4 字节长度头
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("读取消息长度失败: %w", err)
	}

	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen > uint32(MaxMessageSize) {
		return nil, fmt.Errorf("消息长度超出上限: %d 字节", msgLen)
	}
	if msgLen < 1 {
		return nil, fmt.Errorf("消息长度无效: %d 字节", msgLen)
	}

	// 读取消息体（类型 + 载荷）
	body := make([]byte, msgLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("读取消息体失败: %w", err)
	}

	msg := &Message{
		Type:    MessageType(body[0]),
		Payload: make([]byte, msgLen-1),
	}
	copy(msg.Payload, body[1:])

	return msg, nil
}

// WriteMessage 向连接写入一条消息
func WriteMessage(w io.Writer, msg *Message) error {
	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("编码消息失败: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("写入消息失败: %w", err)
	}

	return nil
}
