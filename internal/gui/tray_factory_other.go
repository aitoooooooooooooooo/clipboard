//go:build !darwin

package gui

// newPlatformTray 创建其他平台的托盘实现（空实现）
func newPlatformTray() TrayInterface {
	return newOtherTray()
}
