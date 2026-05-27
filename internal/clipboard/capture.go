package clipboard

import (
	"crypto/sha256"
	"fmt"
)

// ClipboardEvent 剪贴板变更事件
type ClipboardEvent struct {
	ContentType string // "text", "image", "file"
	ContentHash string // SHA-256
	Payload     []byte
	Size        int64
}

// ComputeHash 计算内容的 SHA-256 哈希
func ComputeHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// DetectContentType 检测剪贴板内容类型
// MVP 阶段仅支持文本类型
func DetectContentType(data []byte) string {
	if len(data) == 0 {
		return "text"
	}
	// 检查是否为常见图片格式的魔数
	if len(data) >= 4 {
		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image"
		}
		// GIF: 47 49 46 38
		if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
			return "image"
		}
		// BMP: 42 4D
		if data[0] == 0x42 && data[1] == 0x4D {
			return "image"
		}
	}
	return "text"
}
