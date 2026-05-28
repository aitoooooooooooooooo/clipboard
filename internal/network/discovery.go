package network

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// ServiceType mDNS 服务类型
	ServiceType = "_clipboardsync._tcp"
	// ServiceDomain 服务域
	ServiceDomain = "local."
)

// Discovery 管理 mDNS 服务的广播和发现
type Discovery struct {
	server  *zeroconf.Server
	resolver *zeroconf.Resolver
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewDiscovery 创建发现实例
func NewDiscovery() *Discovery {
	ctx, cancel := context.WithCancel(context.Background())
	return &Discovery{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Advertise 开始广播本机的 ClipboardSync 服务
// 服务类型: _clipboardsync._tcp
// 端口: 可配置（默认 9527）
func (d *Discovery) Advertise(port int, deviceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.server != nil {
		return fmt.Errorf("已在广播中")
	}

	server, err := zeroconf.Register(
		deviceID,            // 实例名
		ServiceType,         // 服务类型
		ServiceDomain,       // 域
		port,                // 端口
		[]string{"device_id=" + deviceID}, // TXT 记录
		nil,                 // 使用默认网络接口
	)
	if err != nil {
		return fmt.Errorf("注册 mDNS 服务失败: %w", err)
	}

	d.server = server
	slog.Info("mDNS 服务广播已启动", "device_id", deviceID, "port", port)

	return nil
}

// Browse 搜索局域网内其他 ClipboardSync 设备
// 返回发现的设备列表（包含 IP、端口、设备 ID）
func (d *Discovery) Browse() ([]*PeerInfo, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("创建 mDNS 解析器失败: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var peers []*PeerInfo
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()

	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			deviceID := ""
			for _, txt := range entry.Text {
				if len(txt) > 10 && txt[:10] == "device_id=" {
					deviceID = txt[10:]
					break
				}
			}

			if deviceID == "" {
				continue
			}

			// 获取 IPv4 地址
			addr := ""
			if len(entry.AddrIPv4) > 0 {
				addr = entry.AddrIPv4[0].String()
			} else if len(entry.AddrIPv6) > 0 {
				addr = entry.AddrIPv6[0].String()
			}

			if addr == "" {
				continue
			}

			peer := &PeerInfo{
				DeviceID: deviceID,
				Address:  addr,
				Port:     entry.Port,
			}

			mu.Lock()
			peers = append(peers, peer)
			mu.Unlock()

			slog.Debug("发现设备", "device_id", deviceID, "addr", addr, "port", entry.Port)
		}
	}(entries)

	if err := resolver.Browse(ctx, ServiceType, ServiceDomain, entries); err != nil {
		return nil, fmt.Errorf("搜索 mDNS 服务失败: %w", err)
	}

	<-ctx.Done()

	mu.Lock()
	result := make([]*PeerInfo, len(peers))
	copy(result, peers)
	mu.Unlock()

	slog.Info("mDNS 发现完成", "devices_found", len(result))

	return result, nil
}

// Stop 停止广播和发现
func (d *Discovery) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.cancel()

	if d.server != nil {
		d.server.Shutdown()
		d.server = nil
		slog.Info("mDNS 服务广播已停止")
	}
}
