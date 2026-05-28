package pairing

import (
	"crypto/ed25519"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/clipboardsync/clipboardsync/internal/storage"
	"github.com/clipboardsync/clipboardsync/pkg/models"
)

// PairingManager 管理配对流程
type PairingManager struct {
	storage    *storage.DB
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	deviceID   string
	sessions   map[string]*PairingSession // key: deviceID
	codes      map[string]*PairingSession // key: pairing code
	mu         sync.RWMutex
}

// NewPairingManager 创建配对管理器
func NewPairingManager(storage *storage.DB, deviceID string) (*PairingManager, error) {
	privateKey, publicKey, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	return &PairingManager{
		storage:    storage,
		privateKey: privateKey,
		publicKey:  publicKey,
		deviceID:   deviceID,
		sessions:   make(map[string]*PairingSession),
		codes:      make(map[string]*PairingSession),
	}, nil
}

// InitiatePairing 发起配对：生成配对码
func (pm *PairingManager) InitiatePairing() (string, error) {
	session, err := GenerateCode(pm.deviceID)
	if err != nil {
		return "", fmt.Errorf("生成配对码失败: %w", err)
	}

	pm.mu.Lock()
	pm.sessions[pm.deviceID] = session
	pm.codes[session.Code] = session
	pm.mu.Unlock()

	slog.Info("配对码已生成", "device_id", pm.deviceID, "expires_at", session.ExpiresAt)
	return session.Code, nil
}

// AcceptPairing 接受配对：通过配对码查找会话，存储对端设备信息
func (pm *PairingManager) AcceptPairing(code string, peerID string, peerPublicKey ed25519.PublicKey) error {
	pm.mu.Lock()
	session, exists := pm.codes[code]
	pm.mu.Unlock()

	if !exists {
		return fmt.Errorf("没有活跃的配对会话")
	}

	if !ValidateCode(session, code) {
		return fmt.Errorf("配对码无效或已过期")
	}

	// 存储对端设备信息（已配对）
	device := &models.Device{
		ID:          peerID,
		DisplayName: peerID, // 初始显示名称使用设备 ID
		Paired:      true,
		LastSeen:    time.Now().UnixMilli(),
		PublicKey:   peerPublicKey,
	}

	if err := pm.storage.SaveDevice(device); err != nil {
		return fmt.Errorf("保存配对设备失败: %w", err)
	}

	// 清除配对会话
	pm.mu.Lock()
	delete(pm.sessions, session.DeviceID)
	delete(pm.codes, code)
	pm.mu.Unlock()

	slog.Info("配对成功", "device_id", pm.deviceID, "peer_id", peerID)
	return nil
}

// IsDevicePaired 检查设备是否已配对
func (pm *PairingManager) IsDevicePaired(deviceID string) (bool, error) {
	device, err := pm.storage.GetDevice(deviceID)
	if err != nil {
		// 设备不存在
		return false, nil
	}
	return device.Paired, nil
}

// GetPairedDevices 获取所有已配对设备
func (pm *PairingManager) GetPairedDevices() ([]*models.Device, error) {
	devices, err := pm.storage.ListDevices()
	if err != nil {
		return nil, fmt.Errorf("获取设备列表失败: %w", err)
	}

	var paired []*models.Device
	for _, d := range devices {
		if d.Paired {
			paired = append(paired, d)
		}
	}
	return paired, nil
}

// GetPublicKey 获取本机公钥
func (pm *PairingManager) GetPublicKey() ed25519.PublicKey {
	return pm.publicKey
}

// FindSessionByCode 根据配对码查找会话（用于 GUI 端接受配对）
func (pm *PairingManager) FindSessionByCode(code string) (*PairingSession, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	session, exists := pm.codes[code]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// Sign 使用本机私钥签名
func (pm *PairingManager) Sign(data []byte) []byte {
	return Sign(pm.privateKey, data)
}

// VerifyPeerSignature 验证对端签名
func (pm *PairingManager) VerifyPeerSignature(peerID string, data, sig []byte) (bool, error) {
	device, err := pm.storage.GetDevice(peerID)
	if err != nil {
		return false, fmt.Errorf("获取对端设备失败: %w", err)
	}

	if !device.Paired {
		return false, fmt.Errorf("设备未配对: %s", peerID)
	}

	publicKey := ed25519.PublicKey(device.PublicKey)
	return Verify(publicKey, data, sig), nil
}
