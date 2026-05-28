//go:build windows

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// hideWindow 隐藏子进程窗口
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// readClipboard 读取当前剪贴板内容
// Windows 使用 powershell，支持文本和文件路径
// 使用单次 PowerShell 调用减少延迟
func readClipboard() ([]byte, error) {
	script := `[Console]::OutputEncoding = [Text.Encoding]::UTF8
$text = Get-Clipboard 2>$null
if ($text) { $text } else {
	$files = Get-Clipboard -Format FileDropList 2>$null
	if ($files) { $files | ForEach-Object { $_.FullName } }
}`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("读取剪贴板失败: %w", err)
	}
	trimmed := strings.TrimRight(string(out), "\r\n")
	if trimmed == "" {
		return nil, fmt.Errorf("剪贴板为空或不支持的格式")
	}
	return []byte(trimmed), nil
}
