// Package cryptox 提供云凭证的 AES-256-GCM 加解密。
// 密钥来自任意字符串，内部经 SHA-256 归一为 32 字节。
package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// Cipher AES-256-GCM 加解密器。
type Cipher struct {
	aead cipher.AEAD
}

// New 根据密钥字符串创建加解密器。
func New(secret string) (*Cipher, error) {
	if secret == "" {
		return nil, errors.New("加密密钥为空")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 明文 → base64(nonce+ciphertext)。
func (c *Cipher) Encrypt(plain string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt base64(nonce+ciphertext) → 明文。
func (c *Cipher) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("解码失败: %w", err)
	}
	size := c.aead.NonceSize()
	if len(raw) < size+1 {
		return "", errors.New("密文长度非法")
	}
	plain, err := c.aead.Open(nil, raw[:size], raw[size:], nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}
	return string(plain), nil
}

// Mask 脱敏展示：保留前 4 位。
func Mask(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****"
}
