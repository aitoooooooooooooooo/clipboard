//go:build windows

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

// readClipboard 读取当前剪贴板文本内容
// Windows 使用 powershell Get-Clipboard
func readClipboard() ([]byte, error) {
	out, err := exec.Command("powershell", "-command", "Get-Clipboard").Output()
	if err != nil {
		return nil, fmt.Errorf("Get-Clipboard 执行失败: %w", err)
	}
	// PowerShell 的 Get-Clipboard 输出包含 CRLF，统一处理
	trimmed := strings.TrimRight(string(out), "\r\n")
	return []byte(trimmed), nil
}

