//go:build darwin

package gui

/*
#include <stdlib.h>

#cgo darwin LDFLAGS: -framework Cocoa

// 外部 C 函数声明
extern void traySetCallback(void (*cb)(int));
extern void trayCreateStatusItem();
extern void traySetTitle(const char* title);
extern void traySetTemplateIcon(const char* iconData, int length);
extern int trayAddMenuItem(const char* title, const char* tooltip, int disabled);
extern void trayAddSeparator();
extern void trayUpdateMenuItemTitle(int menuId, const char* title);
extern void traySetMenuItemChecked(int menuId, int checked);
extern void traySetMenuItemEnabled(int menuId, int enabled);
extern void trayDestroyStatusItem();

// Go 回调函数声明
extern void goMenuItemCallback(int menuId);
*/
import "C"

import (
	"sync"
	"unsafe"
)

// DarwinTray macOS 系统托盘实现
type DarwinTray struct {
	mu        sync.Mutex
	items     map[int]*TrayMenuItem
	callbacks map[int]func()
}

// TrayMenuItem 托盘菜单项
type TrayMenuItem struct {
	id       int
	title    string
	tooltip  string
	disabled bool
	checked  bool
}

// 全局托盘实例
var globalTray *DarwinTray

//export goMenuItemCallback
func goMenuItemCallback(menuId C.int) {
	if globalTray != nil {
		globalTray.handleMenuClick(int(menuId))
	}
}

// 创建新的托盘实例
func newDarwinTray() *DarwinTray {
	tray := &DarwinTray{
		items:     make(map[int]*TrayMenuItem),
		callbacks: make(map[int]func()),
	}
	globalTray = tray
	return tray
}

// 初始化托盘
func (t *DarwinTray) init() {
	C.trayCreateStatusItem()
	// 设置回调函数，将 Go 函数传递给 C
	C.traySetCallback((*[0]byte)(unsafe.Pointer(C.goMenuItemCallback)))
}

// 设置标题
func (t *DarwinTray) setTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.traySetTitle(cTitle)
}

// 设置模板图标
func (t *DarwinTray) setTemplateIcon(iconData []byte) {
	if len(iconData) > 0 {
		C.traySetTemplateIcon((*C.char)(unsafe.Pointer(&iconData[0])), C.int(len(iconData)))
	}
}

// 添加菜单项
func (t *DarwinTray) addMenuItem(title, tooltip string, onClick func()) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	cTitle := C.CString(title)
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cTooltip))

	id := int(C.trayAddMenuItem(cTitle, cTooltip, 0))

	item := &TrayMenuItem{
		id:      id,
		title:   title,
		tooltip: tooltip,
	}
	t.items[id] = item
	t.callbacks[id] = onClick

	return id
}

// 添加分隔符
func (t *DarwinTray) addSeparator() {
	C.trayAddSeparator()
}

// 更新菜单项标题
func (t *DarwinTray) updateMenuItemTitle(id int, title string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, ok := t.items[id]; ok {
		item.title = title
		cTitle := C.CString(title)
		defer C.free(unsafe.Pointer(cTitle))
		C.trayUpdateMenuItemTitle(C.int(id), cTitle)
	}
}

// 设置菜单项选中状态
func (t *DarwinTray) setMenuItemChecked(id int, checked bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, ok := t.items[id]; ok {
		item.checked = checked
		checkedInt := 0
		if checked {
			checkedInt = 1
		}
		C.traySetMenuItemChecked(C.int(id), C.int(checkedInt))
	}
}

// 设置菜单项启用状态
func (t *DarwinTray) setMenuItemEnabled(id int, enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, ok := t.items[id]; ok {
		item.disabled = !enabled
		enabledInt := 0
		if enabled {
			enabledInt = 1
		}
		C.traySetMenuItemEnabled(C.int(id), C.int(enabledInt))
	}
}

// 处理菜单点击
func (t *DarwinTray) handleMenuClick(id int) {
	t.mu.Lock()
	callback, ok := t.callbacks[id]
	t.mu.Unlock()

	if ok && callback != nil {
		go callback()
	}
}

// 销毁托盘
func (t *DarwinTray) destroy() {
	C.trayDestroyStatusItem()
	globalTray = nil
}
