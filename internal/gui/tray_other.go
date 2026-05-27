//go:build !darwin

package gui

// OtherTray 非 macOS 平台的托盘实现（空实现）
type OtherTray struct{}

func newOtherTray() *OtherTray {
	return &OtherTray{}
}

func (t *OtherTray) init()                                        {}
func (t *OtherTray) setTitle(title string)                        {}
func (t *OtherTray) setTemplateIcon(iconData []byte)              {}
func (t *OtherTray) addMenuItem(title, tooltip string, onClick func()) int { return 0 }
func (t *OtherTray) addSeparator()                                {}
func (t *OtherTray) updateMenuItemTitle(id int, title string)     {}
func (t *OtherTray) setMenuItemChecked(id int, checked bool)      {}
func (t *OtherTray) setMenuItemEnabled(id int, enabled bool)      {}
func (t *OtherTray) handleMenuClick(id int)                       {}
func (t *OtherTray) destroy()                                     {}
