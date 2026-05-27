//go:build darwin

#import <Cocoa/Cocoa.h>

// 前向声明
extern void goMenuItemCallback(int menuId);

// 菜单项回调函数类型
typedef void (*MenuItemCallback)(int menuId);

// 全局变量
static NSStatusItem *statusItem = nil;
static NSMenu *trayMenu = nil;
static MenuItemCallback menuCallback = NULL;
static int menuItemCount = 0;

// 设置回调函数
void traySetCallback(MenuItemCallback cb) {
    menuCallback = cb;
}

// 菜单点击处理
@interface TrayMenuHandler : NSObject
- (void)menuItemSelected:(NSMenuItem *)sender;
@end

@implementation TrayMenuHandler
- (void)menuItemSelected:(NSMenuItem *)sender {
    if (menuCallback != NULL) {
        menuCallback((int)[sender tag]);
    }
}
@end

static TrayMenuHandler *handler = nil;

// 创建状态栏项
void trayCreateStatusItem() {
    if (statusItem != nil) return;

    handler = [[TrayMenuHandler alloc] init];
    statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    trayMenu = [[NSMenu alloc] init];
    [trayMenu setAutoenablesItems:FALSE];
    [statusItem setMenu:trayMenu];

    // 设置默认标题
    [statusItem.button setTitle:@"CS"];
}

// 设置标题
void traySetTitle(const char* title) {
    if (statusItem != nil) {
        NSString *nsTitle = [[NSString alloc] initWithCString:title encoding:NSUTF8StringEncoding];
        [statusItem.button setTitle:nsTitle];
    }
}

// 设置模板图标
void traySetTemplateIcon(const char* iconData, int length) {
    if (statusItem != nil && iconData != NULL && length > 0) {
        NSData *data = [[NSData alloc] initWithBytes:iconData length:length];
        NSImage *image = [[NSImage alloc] initWithData:data];
        if (image != nil) {
            [image setTemplate:YES];
            [statusItem.button setImage:image];
        }
    }
}

// 添加菜单项
int trayAddMenuItem(const char* title, const char* tooltip, int disabled) {
    if (trayMenu == nil) return -1;

    NSString *nsTitle = [[NSString alloc] initWithCString:title encoding:NSUTF8StringEncoding];
    NSString *nsTooltip = [[NSString alloc] initWithCString:tooltip encoding:NSUTF8StringEncoding];

    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:nsTitle
                                                  action:@selector(menuItemSelected:)
                                           keyEquivalent:@""];
    [item setTarget:handler];
    [item setTag:menuItemCount];
    [item setToolTip:nsTooltip];

    if (disabled) {
        [item setEnabled:NO];
    }

    [trayMenu addItem:item];
    return menuItemCount++;
}

// 添加分隔符
void trayAddSeparator() {
    if (trayMenu != nil) {
        [trayMenu addItem:[NSMenuItem separatorItem]];
    }
}

// 更新菜单项标题
void trayUpdateMenuItemTitle(int menuId, const char* title) {
    if (trayMenu != nil) {
        NSMenuItem *item = [trayMenu itemWithTag:menuId];
        if (item != nil) {
            NSString *nsTitle = [[NSString alloc] initWithCString:title encoding:NSUTF8StringEncoding];
            [item setTitle:nsTitle];
        }
    }
}

// 设置菜单项选中状态
void traySetMenuItemChecked(int menuId, int checked) {
    if (trayMenu != nil) {
        NSMenuItem *item = [trayMenu itemWithTag:menuId];
        if (item != nil) {
            [item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
        }
    }
}

// 设置菜单项启用状态
void traySetMenuItemEnabled(int menuId, int enabled) {
    if (trayMenu != nil) {
        NSMenuItem *item = [trayMenu itemWithTag:menuId];
        if (item != nil) {
            [item setEnabled:enabled ? YES : NO];
        }
    }
}

// 销毁状态栏项
void trayDestroyStatusItem() {
    if (statusItem != nil) {
        [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
        statusItem = nil;
        trayMenu = nil;
    }
}
