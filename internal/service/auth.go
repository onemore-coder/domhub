// Package service 业务逻辑层。
package service

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/jwtx"
	"github.com/domhub-io/domhub/internal/repo"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserDisabled       = errors.New("账号已被禁用")
)

// AuthService 认证业务。
type AuthService struct {
	users       *repo.UserRepo
	jwtSecret   string
	expireHours int
}

func NewAuthService(users *repo.UserRepo, jwtSecret string, expireHours int) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret, expireHours: expireHours}
}

// Login 校验用户名密码，签发 JWT。
func (s *AuthService) Login(username, password string) (*model.User, string, error) {
	u, err := s.users.FindByUsername(username)
	if err != nil {
		return nil, "", err
	}
	if u == nil {
		return nil, "", ErrInvalidCredentials
	}
	if u.Status != 1 {
		return nil, "", ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := jwtx.GenerateToken(u.ID, u.Username, s.jwtSecret, s.expireHours)
	if err != nil {
		return nil, "", err
	}

	now := time.Now()
	u.LastLoginAt = &now
	if err := s.users.Update(u); err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// GetByID 按 ID 查询用户。
func (s *AuthService) GetByID(id uint) (*model.User, error) {
	return s.users.FindByID(id)
}
