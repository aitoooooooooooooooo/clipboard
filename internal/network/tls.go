package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GenerateSelfSignedCert 生成自签名 TLS 证书（ECDSA P-256）
// 证书有效期 10 年
func GenerateSelfSignedCert(certPath, keyPath string) (tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成 ECDSA 密钥失败: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成序列号失败: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"ClipboardSync"},
			CommonName:   "ClipboardSync Node",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 年
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("创建证书失败: %w", err)
	}

	// 保存证书文件
	if err := os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
		return tls.Certificate{}, fmt.Errorf("创建证书目录失败: %w", err)
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("创建证书文件失败: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return tls.Certificate{}, fmt.Errorf("写入证书失败: %w", err)
	}

	// 保存私钥文件
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("序列化私钥失败: %w", err)
	}

	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("创建私钥文件失败: %w", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return tls.Certificate{}, fmt.Errorf("写入私钥失败: %w", err)
	}

	slog.Info("TLS 证书已生成", "cert", certPath, "key", keyPath)

	// 从文件加载以确保证书格式正确
	return tls.LoadX509KeyPair(certPath, keyPath)
}

// LoadOrCreateCert 加载已有证书或生成新证书
func LoadOrCreateCert(dataDir string) (tls.Certificate, error) {
	certPath := filepath.Join(dataDir, "cert.pem")
	keyPath := filepath.Join(dataDir, "key.pem")

	// 尝试加载已有证书
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				slog.Warn("加载已有证书失败，将重新生成", "error", err)
				return GenerateSelfSignedCert(certPath, keyPath)
			}
			slog.Info("已加载 TLS 证书", "cert", certPath)
			return cert, nil
		}
	}

	return GenerateSelfSignedCert(certPath, keyPath)
}

// TLSConfig 返回 TLS 1.3 配置
// 不验证对端证书（自签名场景）
func TLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		// 自签名证书场景：不验证对端证书
		InsecureSkipVerify: true,
		// 服务器也不验证客户端证书
		ClientAuth: tls.NoClientCert,
	}
}

// TLSServer 监听 TLS 连接
type TLSServer struct {
	listener net.Listener
	config   *tls.Config
	handler  func(net.Conn)
	mu       sync.Mutex
	stopped  bool
}

// NewTLSServer 创建 TLS 服务器
func NewTLSServer(config *tls.Config, handler func(net.Conn)) *TLSServer {
	return &TLSServer{
		config:  config,
		handler: handler,
	}
}

// Start 启动 TLS 服务器，监听指定端口
func (s *TLSServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	listener, err := tls.Listen("tcp", addr, s.config)
	if err != nil {
		return fmt.Errorf("TLS 监听失败: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	slog.Info("TLS 服务器已启动", "addr", addr)

	go s.acceptLoop()

	return nil
}

// acceptLoop 接受连接循环
func (s *TLSServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if stopped {
				return
			}
			slog.Error("接受连接失败", "error", err)
			continue
		}

		slog.Debug("新 TLS 连接", "remote", conn.RemoteAddr())
		go s.handler(conn)
	}
}

// Stop 停止服务器
func (s *TLSServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopped = true
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// TLSClient 发起 TLS 连接到对端
func TLSClient(addr string, config *tls.Config) (net.Conn, error) {
	conn, err := tls.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("TLS 连接失败: %w", err)
	}

	slog.Debug("TLS 连接已建立", "remote", addr)
	return conn, nil
}
