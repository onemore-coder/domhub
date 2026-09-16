// Package totp 实现 RFC 6238 时间型一次性密码（TOTP），
// 兼容 Google Authenticator / 1Password / 腾讯身份验证器等主流 App。
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	stepSeconds = 30
	codeDigits  = 6
	// verifyWindow 允许前后各 1 个时间窗（±30s），容错手机时钟偏差
	verifyWindow = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret 生成 20 字节随机密钥（Base32 无填充，160bit）。
func GenerateSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

// hotp RFC 4226：HMAC-SHA1 截断取 6 位。
func hotp(secret []byte, counter uint64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f)<<24 | uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 | uint32(sum[offset+3])) % 1000000
	return fmt.Sprintf("%06d", code)
}

// Code 计算指定时刻的 TOTP 验证码。
func Code(secretB32 string, t time.Time) (string, error) {
	secret, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secretB32, " ", "")))
	if err != nil {
		return "", fmt.Errorf("无效的 TOTP 密钥: %w", err)
	}
	return hotp(secret, uint64(t.Unix())/stepSeconds), nil
}

// Verify 校验验证码，允许 ±1 个时间窗。
func Verify(secretB32, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != codeDigits {
		return false
	}
	secret, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secretB32, " ", "")))
	if err != nil {
		return false
	}
	counter := uint64(time.Now().Unix()) / stepSeconds
	for i := -verifyWindow; i <= verifyWindow; i++ {
		if hotp(secret, counter+uint64(i)) == code {
			return true
		}
	}
	return false
}

// OtpauthURL 生成验证器 App 可扫描的 otpauth:// URI。
func OtpauthURL(issuer, account, secretB32 string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		issuer, account, secretB32, issuer, codeDigits, stepSeconds)
}
