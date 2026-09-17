// Package service 业务逻辑层。
package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/pkg/cryptox"
	"github.com/onemore-coder/domhub/internal/pkg/jwtx"
	"github.com/onemore-coder/domhub/internal/pkg/totp"
	"github.com/onemore-coder/domhub/internal/repo"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserDisabled       = errors.New("账号已被禁用")
	Err2FARequired        = errors.New("需要两步验证")
	Err2FACode            = errors.New("验证码错误")
	Err2FANotSetup        = errors.New("请先生成两步验证密钥")
)

// AuthService 认证业务。
type AuthService struct {
	users       *repo.UserRepo
	jwtSecret   string
	expireHours int
	cipher      *cryptox.Cipher
}

func NewAuthService(users *repo.UserRepo, jwtSecret string, expireHours int, cipher *cryptox.Cipher) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret, expireHours: expireHours, cipher: cipher}
}

// checkPassword 用户名密码校验（2FA 开启与关闭的登录路径共用）。
func (s *AuthService) checkPassword(username, password string) (*model.User, error) {
	u, err := s.users.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrInvalidCredentials
	}
	if u.Status != 1 {
		return nil, ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

func (s *AuthService) touchLastLogin(u *model.User) {
	now := time.Now()
	u.LastLoginAt = &now
	_ = s.users.Update(u)
}

// Login 校验用户名密码，签发 JWT；开启两步验证的用户返回 Err2FARequired。
func (s *AuthService) Login(username, password string) (*model.User, string, error) {
	u, err := s.checkPassword(username, password)
	if err != nil {
		return nil, "", err
	}
	if u.TotpEnabled {
		return nil, "", Err2FARequired
	}

	token, err := jwtx.GenerateToken(u.ID, u.Username, u.Role, s.jwtSecret, s.expireHours)
	if err != nil {
		return nil, "", err
	}
	s.touchLastLogin(u)
	return u, token, nil
}

// Login2FAPreToken 密码校验通过后签发两步验证中间态令牌（5 分钟有效）。
func (s *AuthService) Login2FAPreToken(username, password string) (string, error) {
	u, err := s.checkPassword(username, password)
	if err != nil {
		return "", err
	}
	return jwtx.Generate2FAPreToken(u.ID, u.Username, s.jwtSecret, 5*time.Minute)
}

// Verify2FALogin 凭预令牌 + 动态码完成登录，签发正式 JWT。
func (s *AuthService) Verify2FALogin(preToken, code string) (*model.User, string, error) {
	claims, err := jwtx.ParseToken(preToken, s.jwtSecret)
	if err != nil || claims.Typ != "2fa" {
		return nil, "", errors.New("验证会话已过期，请重新登录")
	}
	u, err := s.users.FindByID(claims.UserID)
	if err != nil || u == nil {
		return nil, "", ErrInvalidCredentials
	}
	if !u.TotpEnabled || !totp.Verify(s.decryptSecret(u.TotpSecret), code) {
		return nil, "", Err2FACode
	}
	token, err := jwtx.GenerateToken(u.ID, u.Username, u.Role, s.jwtSecret, s.expireHours)
	if err != nil {
		return nil, "", err
	}
	s.touchLastLogin(u)
	return u, token, nil
}

func (s *AuthService) decryptSecret(enc string) string {
	if enc == "" {
		return ""
	}
	plain, err := s.cipher.Decrypt(enc)
	if err != nil {
		return ""
	}
	return plain
}

// Setup2FA 生成 TOTP 密钥（未启用状态），返回二维码 PNG（base64）与密钥明文。
func (s *AuthService) Setup2FA(userID uint) (secret, qrBase64 string, err error) {
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return "", "", errors.New("用户不存在")
	}
	secret, err = totp.GenerateSecret()
	if err != nil {
		return "", "", err
	}
	enc, err := s.cipher.Encrypt(secret)
	if err != nil {
		return "", "", err
	}
	u.TotpSecret = enc
	u.TotpEnabled = false
	if err := s.users.Update(u); err != nil {
		return "", "", err
	}
	png, err := qrcode.Encode(totp.OtpauthURL("DomHub", u.Username, secret), qrcode.Medium, 220)
	if err != nil {
		return "", "", err
	}
	return secret, fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png)), nil
}

// Enable2FA 校验动态码后正式开启两步验证。
func (s *AuthService) Enable2FA(userID uint, code string) error {
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return errors.New("用户不存在")
	}
	secret := s.decryptSecret(u.TotpSecret)
	if secret == "" {
		return Err2FANotSetup
	}
	if !totp.Verify(secret, code) {
		return Err2FACode
	}
	u.TotpEnabled = true
	return s.users.Update(u)
}

// Disable2FA 校验动态码后关闭两步验证。
func (s *AuthService) Disable2FA(userID uint, code string) error {
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return errors.New("用户不存在")
	}
	if !totp.Verify(s.decryptSecret(u.TotpSecret), code) {
		return Err2FACode
	}
	u.TotpSecret = ""
	u.TotpEnabled = false
	return s.users.Update(u)
}

// AdminReset2FA 管理员强制重置某用户的两步验证（丢失验证器时使用）。
func (s *AuthService) AdminReset2FA(userID uint) error {
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return errors.New("用户不存在")
	}
	u.TotpSecret = ""
	u.TotpEnabled = false
	return s.users.Update(u)
}

// GetByID 按 ID 查询用户。
func (s *AuthService) GetByID(id uint) (*model.User, error) {
	return s.users.FindByID(id)
}

// ChangePassword 修改当前用户密码（需校验旧密码）。
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("新密码至少 6 位")
	}
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return errors.New("用户不存在")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)) != nil {
		return errors.New("旧密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.users.Update(u)
}
