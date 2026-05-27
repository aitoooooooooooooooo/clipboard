//go:build darwin

package gui

// newPlatformTray 创建 macOS 平台的托盘实现
func newPlatformTray() TrayInterface {
	return newDarwinTray()
}
