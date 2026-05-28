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
func readClipboard() ([]byte, error) {
	// 先尝试获取文本内容（强制 UTF-8 输出）
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; Get-Clipboard")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err == nil {
		trimmed := strings.TrimRight(string(out), "\r\n")
		if trimmed != "" {
			return []byte(trimmed), nil
		}
	}

	// 文本为空时，尝试获取文件路径列表（CF_HDROP 格式）
	cmd2 := exec.Command("powershell", "-NoProfile", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; Get-Clipboard -Format FileDropList | ForEach-Object { $_.FullName }")
	hideWindow(cmd2)
	out2, err2 := cmd2.Output()
	if err2 == nil {
		trimmed := strings.TrimRight(string(out2), "\r\n")
		if trimmed != "" {
			return []byte(trimmed), nil
		}
	}

	return nil, fmt.Errorf("剪贴板为空或不支持的格式")
}
