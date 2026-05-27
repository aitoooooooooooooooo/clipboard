package gui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/clipboardsync/clipboardsync/internal/clipboard"
	"github.com/clipboardsync/clipboardsync/internal/network"
	"github.com/clipboardsync/clipboardsync/internal/pairing"
	"github.com/clipboardsync/clipboardsync/internal/storage"
	"github.com/clipboardsync/clipboardsync/pkg/models"
	"github.com/google/uuid"
)

// App 是 Wails 应用的后端绑定
type App struct {
	ctx            context.Context
	storage        *storage.DB
	watcher        *clipboard.Watcher
	syncMgr        *network.SyncManager
	pairingMgr     *pairing.PairingManager
	connMgr        *network.ConnectionManager
	discovery      *network.Discovery
	deviceID       string
	dataDir        string
	syncActive     bool
	tray           TrayInterface
	syncMenuItemID int
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Startup Wails 应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.dataDir = dataDir()

	if err := a.initServices(); err != nil {
		slog.Error("服务初始化失败", "error", err)
		return
	}

	// 初始化系统托盘
	a.setupTray()

	slog.Info("应用启动完成", "device_id", a.deviceID)
}

// Shutdown Wails 应用关闭时调用
func (a *App) Shutdown(ctx context.Context) {
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.discovery != nil {
		a.discovery.Stop()
	}
	if a.connMgr != nil {
		a.connMgr.Close()
	}
	if a.storage != nil {
		a.storage.Close()
	}
	slog.Info("应用已关闭")
}

// initServices 初始化所有后端服务
func (a *App) initServices() error {
	// 确保数据目录存在
	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 初始化数据库
	dbPath := filepath.Join(a.dataDir, "clipboardsync.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	a.storage = db

	// 生成或加载设备 ID
	a.deviceID = loadOrCreateDeviceID(a.dataDir)

	// 初始化配对管理器
	pairingMgr, err := pairing.NewPairingManager(a.storage, a.deviceID)
	if err != nil {
		return fmt.Errorf("创建配对管理器失败: %w", err)
	}
	a.pairingMgr = pairingMgr

	// 初始化连接管理器
	a.connMgr = network.NewConnectionManager()

	// 初始化同步管理器
	a.syncMgr = network.NewSyncManager(a.connMgr, a.storage, a.deviceID)
	a.syncMgr.SetOnNewEntry(func(entry *models.ClipboardEntry) {
		slog.Info("收到新同步条目", "entry_id", entry.ID, "source", entry.SourceDevice)
	})

	// 初始化剪贴板监听器
	a.watcher = clipboard.NewWatcher(1000 * time.Millisecond)
	a.watcher.SetOnChange(func(event *clipboard.ClipboardEvent) {
		entry := &models.ClipboardEntry{
			ID:           uuid.New().String(),
			ContentType:  event.ContentType,
			ContentHash:  event.ContentHash,
			Payload:      event.Payload,
			SourceDevice: a.deviceID,
			Timestamp:    time.Now().UnixMilli(),
			Size:         event.Size,
		}

		if err := a.storage.Store(entry); err != nil {
			slog.Debug("存储剪贴板条目失败", "error", err)
			return
		}

		if a.syncActive {
			if err := a.syncMgr.BroadcastEntry(entry); err != nil {
				slog.Warn("广播条目失败", "error", err)
			}
		}
	})

	// 初始化网络发现
	a.discovery = network.NewDiscovery()

	return nil
}

// --- 历史管理 ---

// GetHistory 获取剪贴板历史（分页）
func (a *App) GetHistory(limit int) ([]*models.ClipboardEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	entries, err := a.storage.List(limit)
	if err != nil {
		return nil, fmt.Errorf("获取历史失败: %w", err)
	}
	return entries, nil
}

// SearchHistory 搜索历史
func (a *App) SearchHistory(query string) ([]*models.ClipboardEntry, error) {
	entries, err := a.storage.Search(query, 50)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	return entries, nil
}

// DeleteEntry 删除条目
func (a *App) DeleteEntry(id string) error {
	if err := a.storage.Delete(id); err != nil {
		return fmt.Errorf("删除条目失败: %w", err)
	}
	return nil
}

// ClearHistory 清空历史
func (a *App) ClearHistory() error {
	entries, err := a.storage.List(10000)
	if err != nil {
		return fmt.Errorf("获取条目失败: %w", err)
	}
	for _, entry := range entries {
		if err := a.storage.Delete(entry.ID); err != nil {
			slog.Warn("删除条目失败", "id", entry.ID, "error", err)
		}
	}
	return nil
}

// --- 设备管理 ---

// GetDevices 获取已配对设备列表
func (a *App) GetDevices() ([]*models.Device, error) {
	devices, err := a.pairingMgr.GetPairedDevices()
	if err != nil {
		return nil, fmt.Errorf("获取设备列表失败: %w", err)
	}
	return devices, nil
}

// GeneratePairingCode 生成配对码
func (a *App) GeneratePairingCode() (string, error) {
	code, err := a.pairingMgr.InitiatePairing()
	if err != nil {
		return "", fmt.Errorf("生成配对码失败: %w", err)
	}
	return code, nil
}

// AcceptPairing 接受配对
func (a *App) AcceptPairing(code string) error {
	// 需要对端的设备 ID 和公钥，这里简化处理
	// 实际应通过网络协议获取
	return fmt.Errorf("请通过网络协议完成配对")
}

// --- 同步控制 ---

// ToggleSync 开关同步
func (a *App) ToggleSync(enabled bool) error {
	if enabled {
		// 启动剪贴板监听
		a.watcher.Start()
		// 启动 mDNS 广播
		if err := a.discovery.Advertise(9527, a.deviceID); err != nil {
			slog.Warn("mDNS 广播启动失败", "error", err)
		}
		a.syncActive = true
		slog.Info("同步已启用")
	} else {
		a.watcher.Stop()
		a.discovery.Stop()
		a.syncActive = false
		slog.Info("同步已禁用")
	}
	return nil
}

// IsSyncActive 是否正在同步
func (a *App) IsSyncActive() bool {
	return a.syncActive
}

// --- 应用信息 ---

// GetContext 获取应用上下文（供托盘菜单使用）
func (a *App) GetContext() context.Context {
	return a.ctx
}

// GetDeviceID 获取本机设备 ID
func (a *App) GetDeviceID() string {
	return a.deviceID
}

// --- 内部辅助函数 ---

// dataDir 返回应用数据目录
func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".clipboardsync"
	}
	return filepath.Join(home, ".clipboardsync")
}

// loadOrCreateDeviceID 加载或生成设备 ID
func loadOrCreateDeviceID(dataDir string) string {
	idFile := filepath.Join(dataDir, "device_id")

	// 尝试读取已有 ID
	data, err := os.ReadFile(idFile)
	if err == nil && len(data) > 0 {
		return string(data)
	}

	// 生成新 ID
	id := uuid.New().String()
	if err := os.WriteFile(idFile, []byte(id), 0644); err != nil {
		slog.Warn("保存设备 ID 失败", "error", err)
	}
	return id
}
