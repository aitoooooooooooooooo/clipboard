package pairing

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// PairingSession 配对会话
type PairingSession struct {
	Code      string
	ExpiresAt time.Time
	DeviceID  string
}

// GenerateCode 生成 6 位随机数字配对码，有效期 60 秒
func GenerateCode(deviceID string) (*PairingSession, error) {
	// 生成 6 位随机数字
	code, err := generateNumericCode(6)
	if err != nil {
		return nil, fmt.Errorf("生成配对码失败: %w", err)
	}

	session := &PairingSession{
		Code:      code,
		ExpiresAt: time.Now().Add(60 * time.Second),
		DeviceID:  deviceID,
	}

	return session, nil
}

// ValidateCode 验证配对码（未过期且匹配）
func ValidateCode(session *PairingSession, code string) bool {
	if session == nil {
		return false
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return false
	}

	// 检查配对码是否匹配
	return session.Code == code
}

// generateNumericCode 生成指定位数的随机数字字符串
func generateNumericCode(digits int) (string, error) {
	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}

	// 补零到指定位数
	return fmt.Sprintf("%0*d", digits, n), nil
}
