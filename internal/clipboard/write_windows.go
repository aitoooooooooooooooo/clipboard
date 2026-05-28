//go:build windows

package clipboard

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// writeText 将文本写入系统剪贴板
func writeText(data []byte) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; $input | Set-Clipboard")
	hideWindow(cmd)
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入文本到剪贴板失败: %s: %w", string(output), err)
	}
	return nil
}

// writeImage 将图片数据写入系统剪贴板
func writeImage(data []byte) error {
	// Windows: 使用 PowerShell + System.Windows.Forms 写入图片
	// 先将数据保存到临时文件，再加载到剪贴板
	script := `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
try {
    $bytes = [System.Console]::OpenStandardInput()
    $ms = New-Object System.IO.MemoryStream
    $buffer = New-Object byte[] 65536
    do {
        $count = $bytes.Read($buffer, 0, $buffer.Length)
        if ($count -gt 0) { $ms.Write($buffer, 0, $count) }
    } while ($count -gt 0)
    $ms.Position = 0
    $img = [System.Drawing.Image]::FromStream($ms)
    [System.Windows.Forms.Clipboard]::SetImage($img)
    $img.Dispose()
    $ms.Dispose()
    $bytes.Dispose()
} catch {
    Write-Error $_.Exception.Message
    exit 1
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	hideWindow(cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入图片到剪贴板失败: %s: %w", string(output), err)
	}
	return nil
}

// writeFiles 将文件路径列表写入系统剪贴板
func writeFiles(data []byte) error {
	// Windows: 使用 PowerShell 的 Set-Clipboard -Path
	paths := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(paths) == 0 {
		return fmt.Errorf("没有文件路径")
	}

	// 过滤空路径
	var validPaths []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			validPaths = append(validPaths, p)
		}
	}
	if len(validPaths) == 0 {
		return fmt.Errorf("没有有效的文件路径")
	}

	// 构建 PowerShell 命令
	pathArgs := strings.Join(validPaths, ",")
	script := fmt.Sprintf("Set-Clipboard -Path @(%s)", pathArgs)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入文件到剪贴板失败: %s: %w", string(output), err)
	}
	return nil
}
