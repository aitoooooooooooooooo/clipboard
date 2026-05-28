//go:build darwin

package clipboard

// clipboardState macOS 剪贴板状态（始终使用哈希比较）
type clipboardState struct{}

// newClipboardState 创建剪贴板状态
func newClipboardState() *clipboardState {
	return &clipboardState{}
}

// changed macOS 上始终返回 true（pbpaste 很快，直接读取比较）
func (s *clipboardState) changed() bool {
	return true
}
