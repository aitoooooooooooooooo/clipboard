package clipboard

import "fmt"

// WriteToClipboard 将内容写入系统剪贴板
// contentType: "text", "image", "file"
// payload: 内容数据
func WriteToClipboard(contentType string, payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("内容为空")
	}

	switch contentType {
	case "text":
		return writeText(payload)
	case "image":
		return writeImage(payload)
	case "file":
		return writeFiles(payload)
	default:
		return fmt.Errorf("不支持的内容类型: %s", contentType)
	}
}
