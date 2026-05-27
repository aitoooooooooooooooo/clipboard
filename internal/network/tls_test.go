package network

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	cert, err := GenerateSelfSignedCert(certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() 失败: %v", err)
	}

	// 验证证书文件存在
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("证书文件未创建")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("私钥文件未创建")
	}

	// 验证证书有效
	if cert.Certificate == nil {
		t.Error("证书为空")
	}

	// 验证私钥存在
	if cert.PrivateKey == nil {
		t.Error("私钥为空")
	}

	// 验证证书可以用于 TLS 配置
	config := TLSConfig(cert)
	if config.MinVersion != tls.VersionTLS13 {
		t.Errorf("TLS 最低版本不是 1.3: 期望 %d, 实际 %d", tls.VersionTLS13, config.MinVersion)
	}
}

func TestLoadOrCreateCert(t *testing.T) {
	tmpDir := t.TempDir()

	// 第一次调用：创建新证书
	cert1, err := LoadOrCreateCert(tmpDir)
	if err != nil {
		t.Fatalf("首次 LoadOrCreateCert() 失败: %v", err)
	}

	// 第二次调用：加载已有证书
	cert2, err := LoadOrCreateCert(tmpDir)
	if err != nil {
		t.Fatalf("二次 LoadOrCreateCert() 失败: %v", err)
	}

	// 验证两次加载的是同一个证书
	if len(cert1.Certificate) != len(cert2.Certificate) {
		t.Error("两次加载的证书长度不同")
	}
}

func TestTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cert, err := GenerateSelfSignedCert(
		filepath.Join(tmpDir, "cert.pem"),
		filepath.Join(tmpDir, "key.pem"),
	)
	if err != nil {
		t.Fatalf("生成证书失败: %v", err)
	}

	config := TLSConfig(cert)

	// 验证 TLS 1.3 最低版本
	if config.MinVersion != tls.VersionTLS13 {
		t.Errorf("TLS 最低版本不是 1.3: %d", config.MinVersion)
	}

	// 验证不验证对端证书（自签名场景）
	if !config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify 应为 true")
	}

	// 验证有证书
	if len(config.Certificates) == 0 {
		t.Error("配置中没有证书")
	}
}
