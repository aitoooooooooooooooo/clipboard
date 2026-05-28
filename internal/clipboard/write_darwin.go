//go:build darwin

package clipboard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	imageHelperPath string
	imageHelperOnce sync.Once
)

// writeText 将文本写入系统剪贴板
func writeText(data []byte) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}

// writeImage 将图片数据写入系统剪贴板
func writeImage(data []byte) error {
	helper := getImageHelper()
	if helper == "" {
		return fmt.Errorf("图片复制不可用（需要 Python3 + pyobjc）")
	}
	cmd := exec.Command(helper)
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入图片到剪贴板失败: %s: %w", string(output), err)
	}
	return nil
}

// writeFiles 将文件路径列表写入系统剪贴板
func writeFiles(data []byte) error {
	// data 包含用换行分隔的文件路径
	// 使用 osascript 将文件引用写入剪贴板
	script := fmt.Sprintf(`
set filePaths to "%s"
set oldDelimiters to AppleScript's text item delimiters
set AppleScript's text item delimiters to "
"
set pathList to every text item of filePaths
set fileList to {}
repeat with p in pathList
	if p is not "" then
		set end of fileList to (POSIX file p as alias)
	end if
end repeat
set the clipboard to fileList
set AppleScript's text item delimiters to oldDelimiters
`, string(data))

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入文件到剪贴板失败: %s: %w", string(output), err)
	}
	return nil
}

// getImageHelper 获取图片写入辅助程序路径
func getImageHelper() string {
	imageHelperOnce.Do(func() {
		home, _ := os.UserHomeDir()
		helperDir := filepath.Join(home, ".clipboardsync", "bin")
		os.MkdirAll(helperDir, 0755)
		helperPath := filepath.Join(helperDir, "clipboard_img_write")

		// 检查是否已编译
		if _, err := os.Stat(helperPath); err == nil {
			imageHelperPath = helperPath
			return
		}

		// 编译 Objective-C 辅助程序
		srcPath := filepath.Join(helperDir, "clipboard_img_write.m")
		src := `#import <Cocoa/Cocoa.h>
int main() {
    @autoreleasepool {
        NSData *input = [NSFileHandle fileHandleWithStandardInput];
        NSData *data = [input readDataToEndOfFile];
        if ([data length] == 0) { return 1; }
        NSImage *image = [[NSImage alloc] initWithData:data];
        if (!image) { return 1; }
        NSArray *types = @[NSPasteboardTypePNG];
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        [pb setData:data forType:NSPasteboardTypePNG];
    }
    return 0;
}
`
		if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
			return
		}

		cmd := exec.Command("clang",
			"-framework", "Cocoa",
			"-fobjc-arc",
			"-O2", "-o", helperPath, srcPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			// 编译失败，尝试用 Python 作为备用
			_ = output
			imageHelperPath = tryPythonHelper(helperDir)
			return
		}

		imageHelperPath = helperPath
	})
	return imageHelperPath
}

// tryPythonHelper 尝试创建 Python 备用脚本
func tryPythonHelper(helperDir string) string {
	scriptPath := filepath.Join(helperDir, "clipboard_img_write.py")
	script := `#!/usr/bin/env python3
import sys
try:
    from AppKit import NSPasteboard, NSPasteboardTypePNG, NSImage, NSData
    data = sys.stdin.buffer.read()
    ns_data = NSData.dataWithBytes_length_(data, len(data))
    image = NSImage.alloc().initWithData_(ns_data)
    if not image:
        sys.exit(1)
    pb = NSPasteboard.generalPasteboard()
    pb.clearContents()
    pb.setData_forType_(ns_data, NSPasteboardTypePNG)
except Exception:
    sys.exit(1)
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return ""
	}
	// 测试 Python3 + pyobjc 是否可用
	cmd := exec.Command("python3", "-c", "from AppKit import NSPasteboard")
	if cmd.Run() != nil {
		return ""
	}
	return scriptPath
}

// runImageHelper 执行图片写入辅助程序
func runImageHelper(data []byte) error {
	helper := getImageHelper()
	if helper == "" {
		return fmt.Errorf("图片复制不可用")
	}

	var cmd *exec.Cmd
	if filepath.Ext(helper) == ".py" {
		cmd = exec.Command("python3", helper)
	} else {
		cmd = exec.Command(helper)
	}
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入图片失败: %s: %w", string(output), err)
	}
	return nil
}
