//go:build darwin

package clipboard

import (
	"fmt"
	"os/exec"
)

// readClipboard 读取当前剪贴板文本内容
// macOS 使用 pbpaste 命令
func readClipboard() ([]byte, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return nil, fmt.Errorf("pbpaste 执行失败: %w", err)
	}
	return out, nil
}

