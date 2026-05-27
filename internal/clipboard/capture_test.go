package clipboard

import (
	"testing"
)

func TestComputeHash(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "空内容",
			input:    []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "简单文本",
			input:    []byte("hello"),
			expected: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			name:     "中文文本",
			input:    []byte("你好世界"),
			expected: "", // 仅验证长度和一致性
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeHash(tt.input)
			if len(result) != 64 {
				t.Errorf("哈希长度应为 64，实际为 %d", len(result))
			}
			if tt.expected != "" && result != tt.expected {
				t.Errorf("ComputeHash(%v) = %s, 期望 %s", tt.input, result, tt.expected)
			}
			// 验证相同输入产生相同哈希
			result2 := ComputeHash(tt.input)
			if result != result2 {
				t.Errorf("相同输入应产生相同哈希: %s != %s", result, result2)
			}
		})
	}

	// 验证不同输入产生不同哈希
	hash1 := ComputeHash([]byte("hello"))
	hash2 := ComputeHash([]byte("world"))
	if hash1 == hash2 {
		t.Error("不同输入应产生不同哈希")
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "空内容默认文本",
			input:    []byte{},
			expected: "text",
		},
		{
			name:     "普通文本",
			input:    []byte("hello world"),
			expected: "text",
		},
		{
			name:     "PNG 图片",
			input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expected: "image",
		},
		{
			name:     "JPEG 图片",
			input:    []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
			expected: "image",
		},
		{
			name:     "GIF 图片",
			input:    []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
			expected: "image",
		},
		{
			name:     "BMP 图片",
			input:    []byte{0x42, 0x4D, 0x36, 0x00},
			expected: "image",
		},
		{
			name:     "短文本不误判",
			input:    []byte{0x89, 0x50},
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectContentType(tt.input)
			if result != tt.expected {
				t.Errorf("DetectContentType(%v) = %s, 期望 %s", tt.input, result, tt.expected)
			}
		})
	}
}
