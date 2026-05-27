package pairing

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

// GenerateKeyPair 生成 Ed25519 密钥对
func GenerateKeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("生成 Ed25519 密钥对失败: %w", err)
	}
	return privateKey, publicKey, nil
}

// Sign 使用私钥签名
func Sign(privateKey ed25519.PrivateKey, data []byte) []byte {
	return ed25519.Sign(privateKey, data)
}

// Verify 使用公钥验证签名
func Verify(publicKey ed25519.PublicKey, data, sig []byte) bool {
	return ed25519.Verify(publicKey, data, sig)
}
