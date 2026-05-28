//go:build windows

package clipboard

import "syscall"

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	getClipboardSeqNo = user32.NewProc("GetClipboardSequenceNumber")
)

// clipboardState Windows 剪贴板状态（序列号）
type clipboardState struct {
	seqNo uint32
}

// newClipboardState 创建剪贴板状态
func newClipboardState() *clipboardState {
	return &clipboardState{
		seqNo: getClipboardSeqNum(),
	}
}

// changed 检查剪贴板是否变化（Windows 使用序列号，微秒级）
func (s *clipboardState) changed() bool {
	current := getClipboardSeqNum()
	if current != s.seqNo {
		s.seqNo = current
		return true
	}
	return false
}

// getClipboardSeqNum 获取剪贴板序列号
func getClipboardSeqNum() uint32 {
	ret, _, _ := getClipboardSeqNo.Call()
	return uint32(ret)
}
