package gui

import (
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TrayInterface 托盘接口
type TrayInterface interface {
	init()
	setTitle(title string)
	setTemplateIcon(iconData []byte)
	addMenuItem(title, tooltip string, onClick func()) int
	addSeparator()
	updateMenuItemTitle(id int, title string)
	setMenuItemChecked(id int, checked bool)
	setMenuItemEnabled(id int, enabled bool)
	destroy()
}

// setupTray 初始化系统托盘
func (a *App) setupTray() {
	// 创建平台特定的托盘实现
	a.tray = newPlatformTray()
	a.tray.init()

	// 设置标题
	a.tray.setTitle("ClipboardSync")

	// 添加菜单项
	a.setupTrayMenu()
}

// setupTrayMenu 设置托盘菜单
func (a *App) setupTrayMenu() {
	// 打开窗口
	mShow := a.tray.addMenuItem("打开窗口", "显示主窗口", func() {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
	})
	_ = mShow

	a.tray.addSeparator()

	// 同步开关
	a.syncMenuItemID = a.tray.addMenuItem("启用同步", "开启/关闭剪贴板同步", func() {
		a.ToggleSync(!a.syncActive)
		a.updateSyncMenu()
	})
	a.updateSyncMenu()

	// 设备管理
	a.tray.addMenuItem("设备管理", "管理配对设备", func() {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
	})

	a.tray.addSeparator()

	// 清空历史
	a.tray.addMenuItem("清空历史", "清空剪贴板历史", func() {
		if err := a.ClearHistory(); err != nil {
			slog.Error("清空历史失败", "error", err)
		}
	})

	a.tray.addSeparator()

	// 退出
	a.tray.addMenuItem("退出", "退出应用", func() {
		a.tray.destroy()
		runtime.Quit(a.ctx)
	})
}

// updateSyncMenu 更新同步菜单项的状态
func (a *App) updateSyncMenu() {
	if a.syncActive {
		a.tray.updateMenuItemTitle(a.syncMenuItemID, "禁用同步")
		a.tray.setMenuItemChecked(a.syncMenuItemID, true)
	} else {
		a.tray.updateMenuItemTitle(a.syncMenuItemID, "启用同步")
		a.tray.setMenuItemChecked(a.syncMenuItemID, false)
	}
}

// ShutdownTray 关闭托盘
func (a *App) ShutdownTray() {
	if a.tray != nil {
		a.tray.destroy()
	}
}
