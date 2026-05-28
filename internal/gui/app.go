package gui

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/clipboardsync/clipboardsync/internal/clipboard"
	"github.com/clipboardsync/clipboardsync/internal/network"
	"github.com/clipboardsync/clipboardsync/internal/pairing"
	"github.com/clipboardsync/clipboardsync/internal/storage"
	"github.com/clipboardsync/clipboardsync/pkg/models"
	"github.com/google/uuid"
)

// DiscoveredDevice 已发现但未配对的设备
type DiscoveredDevice struct {
	DeviceID string `json:"device_id"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
}

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
	startupErr     string // 启动错误信息，供前端查询

	// TLS 和对端连接
	tlsServer      *network.TLSServer
	tlsConfig      *tls.Config
	stopCh         chan struct{}
	discoverWg     sync.WaitGroup
	discovered     map[string]*DiscoveredDevice
	discoveredMu   sync.RWMutex
	pendingPubKey  []byte // 发送配对请求时暂存本机公钥
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{
		discovered: make(map[string]*DiscoveredDevice),
	}
}

// Startup Wails 应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.dataDir = dataDir()

	if err := a.initServices(); err != nil {
		errMsg := fmt.Sprintf("服务初始化失败: %v", err)
		slog.Error(errMsg)
		a.startupErr = errMsg
		return
	}

	// 初始化系统托盘
	a.setupTray()

	// 启动网络层（TLS + mDNS 广播 + 设备发现）
	a.startNetworking()

	slog.Info("应用启动完成", "device_id", a.deviceID)
}

// GetStartupError 获取启动错误信息（前端调用以显示具体错误）
func (a *App) GetStartupError() string {
	return a.startupErr
}

// Shutdown Wails 应用关闭时调用
func (a *App) Shutdown(ctx context.Context) {
	// 停止对端发现循环
	if a.stopCh != nil {
		close(a.stopCh)
	}
	a.discoverWg.Wait()

	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.tlsServer != nil {
		a.tlsServer.Stop()
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

	// 注册配对请求处理器（当其他设备请求配对时）
	a.syncMgr.SetOnPairingRequest(func(req *network.PairingRequestPayload) *network.PairingResponsePayload {
		// 验证配对码
		session, valid := a.pairingMgr.FindSessionByCode(req.Code)
		if !valid {
			return &network.PairingResponsePayload{
				Accepted: false,
				DeviceID: a.deviceID,
				Error:    "配对码无效或已过期",
			}
		}
		_ = session

		// 存储对端设备
		peerDevice := &models.Device{
			ID:          req.DeviceID,
			DisplayName: req.DeviceID,
			Paired:      true,
			LastSeen:    time.Now().UnixMilli(),
			PublicKey:   req.PublicKey,
		}
		if err := a.storage.SaveDevice(peerDevice); err != nil {
			slog.Error("保存配对设备失败", "error", err)
			return &network.PairingResponsePayload{
				Accepted: false,
				DeviceID: a.deviceID,
				Error:    "保存设备失败",
			}
		}

		slog.Info("配对请求已接受", "peer", req.DeviceID, "code", req.Code)
		return &network.PairingResponsePayload{
			Accepted: true,
			DeviceID: a.deviceID,
		}
	})

	// 注册配对响应处理器（当本机发送配对请求收到响应时）
	a.syncMgr.SetOnPairingResponse(func(resp *network.PairingResponsePayload) {
		if !resp.Accepted {
			slog.Info("配对被拒绝", "reason", resp.Error)
			return
		}

		// 保存对端设备信息（请求方也需要保存响应方）
		peerDevice := &models.Device{
			ID:          resp.DeviceID,
			DisplayName: resp.DeviceID,
			Paired:      true,
			LastSeen:    time.Now().UnixMilli(),
			PublicKey:   a.pendingPubKey,
		}
		if err := a.storage.SaveDevice(peerDevice); err != nil {
			slog.Error("保存配对设备失败", "error", err)
			return
		}
		a.pendingPubKey = nil
		slog.Info("配对完成，已保存对端设备", "device_id", resp.DeviceID)
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

	// 初始化 TLS 证书和服务器
	cert, err := network.LoadOrCreateCert(a.dataDir)
	if err != nil {
		return fmt.Errorf("加载 TLS 证书失败: %w", err)
	}
	a.tlsConfig = network.TLSConfig(cert)
	a.tlsServer = network.NewTLSServer(a.tlsConfig, a.handleIncomingConnection)
	a.stopCh = make(chan struct{})

	return nil
}

// --- 历史管理 ---

// GetHistory 获取剪贴板历史（分页）
func (a *App) GetHistory(limit int) ([]*models.ClipboardEntry, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
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
	if a.storage == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	entries, err := a.storage.Search(query, 50)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	return entries, nil
}

// DeleteEntry 删除条目
func (a *App) DeleteEntry(id string) error {
	if a.storage == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := a.storage.Delete(id); err != nil {
		return fmt.Errorf("删除条目失败: %w", err)
	}
	return nil
}

// CopyEntryToClipboard 将指定条目复制到系统剪贴板
func (a *App) CopyEntryToClipboard(id string) error {
	if a.storage == nil {
		return fmt.Errorf("数据库未初始化")
	}
	entry, err := a.storage.Get(id)
	if err != nil {
		return fmt.Errorf("获取条目失败: %w", err)
	}
	if err := clipboard.WriteToClipboard(entry.ContentType, entry.Payload); err != nil {
		return fmt.Errorf("写入剪贴板失败: %w", err)
	}
	return nil
}

// ClearHistory 清空历史
func (a *App) ClearHistory() error {
	if a.storage == nil {
		return fmt.Errorf("数据库未初始化")
	}
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
	if a.pairingMgr == nil {
		return nil, fmt.Errorf("配对管理器未初始化")
	}
	devices, err := a.pairingMgr.GetPairedDevices()
	if err != nil {
		return nil, fmt.Errorf("获取设备列表失败: %w", err)
	}
	return devices, nil
}

// GeneratePairingCode 生成配对码
func (a *App) GeneratePairingCode() (string, error) {
	if a.pairingMgr == nil {
		return "", fmt.Errorf("配对管理器未初始化")
	}
	code, err := a.pairingMgr.InitiatePairing()
	if err != nil {
		return "", fmt.Errorf("生成配对码失败: %w", err)
	}
	return code, nil
}

// AcceptPairing 接受配对：通过网络向配对码生成方发送配对请求
func (a *App) AcceptPairing(code string) error {
	if a.pairingMgr == nil {
		return fmt.Errorf("配对管理器未初始化")
	}
	if a.syncMgr == nil {
		return fmt.Errorf("同步管理器未初始化")
	}
	if len(code) != 6 {
		return fmt.Errorf("配对码必须是 6 位数字")
	}

	// 检查是否有已连接的对端
	peers := a.connMgr.List()
	if len(peers) == 0 {
		return fmt.Errorf("没有已连接的设备，请先启用同步并确保对方在线")
	}

	// 获取本机公钥
	publicKey := a.pairingMgr.GetPublicKey()
	a.pendingPubKey = publicKey

	// 向所有已连接对端发送配对请求
	if err := a.syncMgr.SendPairingRequest(code, a.deviceID, publicKey); err != nil {
		return fmt.Errorf("发送配对请求失败: %w", err)
	}

	slog.Info("配对请求已发送", "code", code, "peer_count", len(peers))
	return nil
}

// --- 网络层 ---

// startNetworking 启动网络层（TLS 服务器 + mDNS 广播 + 设备发现）
func (a *App) startNetworking() {
	// 启动 TLS 服务器
	if err := a.tlsServer.Start(9527); err != nil {
		slog.Warn("TLS 服务器启动失败", "error", err)
	}

	// 启动 mDNS 广播（让其他设备能发现本机）
	if err := a.discovery.Advertise(9527, a.deviceID); err != nil {
		slog.Warn("mDNS 广播启动失败", "error", err)
	}

	// 启动对端发现和自动连接循环
	a.discoverWg.Add(1)
	go a.discoverAndConnect()

	// 立即执行一次发现（不等待定时器）
	go a.doDiscover()

	slog.Info("网络层已启动", "port", 9527)
}

// --- 同步控制 ---

// ToggleSync 开关剪贴板同步监听
func (a *App) ToggleSync(enabled bool) error {
	if a.watcher == nil {
		return fmt.Errorf("服务未初始化")
	}
	if enabled {
		// 启动剪贴板监听
		a.watcher.Start()
		a.syncActive = true
		slog.Info("剪贴板同步已启用")
	} else {
		a.watcher.Stop()
		a.syncActive = false
		slog.Info("剪贴板同步已禁用")
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

// --- 对端连接管理 ---

// handleIncomingConnection 处理传入的 TLS 连接
func (a *App) handleIncomingConnection(conn net.Conn) {
	slog.Info("收到传入连接", "remote", conn.RemoteAddr())

	// 读取第一条消息以识别对端身份
	msg, err := network.ReadMessage(conn)
	if err != nil {
		slog.Warn("读取对端身份失败", "error", err)
		conn.Close()
		return
	}

	// 简单的身份识别：使用远程地址作为临时 ID
	peerID := conn.RemoteAddr().String()

	peer := &network.PeerInfo{
		DeviceID: peerID,
		Conn:     conn,
		Address:  peerID,
	}

	a.connMgr.Add(peer)

	// 处理已读取的第一条消息
	if err := a.syncMgr.HandleMessage(peerID, msg); err != nil {
		slog.Warn("处理消息失败", "from", peerID, "error", err)
	}

	// 继续读取消息
	a.handlePeerMessages(peerID, conn)
}

// connectToPeer 连接到对端
func (a *App) connectToPeer(peer *network.PeerInfo) error {
	// 检查是否已连接
	if _, exists := a.connMgr.Get(peer.DeviceID); exists {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
	conn, err := network.TLSClient(addr, a.tlsConfig)
	if err != nil {
		return fmt.Errorf("连接到 %s 失败: %w", addr, err)
	}

	peer.Conn = conn
	a.connMgr.Add(peer)

	// 启动消息读取
	go a.handlePeerMessages(peer.DeviceID, conn)

	// 连接成功后请求历史
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := a.syncMgr.RequestHistory(peer.DeviceID); err != nil {
			slog.Warn("请求历史失败", "peer", peer.DeviceID, "error", err)
		}
	}()

	return nil
}

// handlePeerMessages 循环读取对端消息
func (a *App) handlePeerMessages(peerID string, conn net.Conn) {
	defer func() {
		a.connMgr.Remove(peerID)
	}()

	for {
		msg, err := network.ReadMessage(conn)
		if err != nil {
			slog.Debug("对端连接断开", "peer", peerID, "error", err)
			return
		}

		if err := a.syncMgr.HandleMessage(peerID, msg); err != nil {
			slog.Warn("处理消息失败", "from", peerID, "error", err)
		}
	}
}

// discoverAndConnect 定期发现并连接对端
func (a *App) discoverAndConnect() {
	defer a.discoverWg.Done()

	// 首次发现
	a.doDiscover()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.doDiscover()
		}
	}
}

// doDiscover 执行一次对端发现和连接
func (a *App) doDiscover() {
	peers, err := a.discovery.Browse()
	if err != nil {
		slog.Debug("发现对端失败", "error", err)
		return
	}

	for _, peer := range peers {
		// 跳过自己
		if peer.DeviceID == a.deviceID {
			continue
		}

		// 记录发现的设备
		a.discoveredMu.Lock()
		a.discovered[peer.DeviceID] = &DiscoveredDevice{
			DeviceID: peer.DeviceID,
			Address:  peer.Address,
			Port:     peer.Port,
		}
		a.discoveredMu.Unlock()

		if err := a.connectToPeer(peer); err != nil {
			slog.Debug("连接对端失败", "device_id", peer.DeviceID, "error", err)
		}
	}
}

// GetDiscoveredDevices 获取局域网内已发现的设备列表
func (a *App) GetDiscoveredDevices() ([]*DiscoveredDevice, error) {
	a.discoveredMu.RLock()
	defer a.discoveredMu.RUnlock()

	devices := make([]*DiscoveredDevice, 0, len(a.discovered))
	for _, d := range a.discovered {
		devices = append(devices, d)
	}
	return devices, nil
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
