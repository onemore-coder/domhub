package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/repo"
)

// TokenService API 令牌业务：签发（明文仅返回一次）、列表、吊销、鉴权查找。
type TokenService struct {
	tokens *repo.ApiTokenRepo
	users  *repo.UserRepo
}

func NewTokenService(tokens *repo.ApiTokenRepo, users *repo.UserRepo) *TokenService {
	return &TokenService{tokens: tokens, users: users}
}

// CreateInput 签发参数。
type CreateInput struct {
	UserID     uint
	Username   string
	Name       string
	ExpireDays int // 0 = 永不过期
}

// CreateResult 签发结果（Token 明文仅此一次返回）。
type CreateResult struct {
	Token *model.ApiToken `json:"token"`
	Plain string          `json:"plain_token"`
}

// Create 签发新令牌。
func (s *TokenService) Create(in CreateInput) (*CreateResult, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, errors.New("令牌名称不能为空")
	}
	if in.ExpireDays < 0 || in.ExpireDays > 3650 {
		return nil, errors.New("有效期须在 0~3650 天之间（0 = 永不过期）")
	}

	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("生成随机数失败: %w", err)
	}
	plain := model.ApiTokenPrefix + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))

	t := &model.ApiToken{
		UserID:    in.UserID,
		Name:      in.Name,
		TokenHash: hex.EncodeToString(sum[:]),
		Prefix:    plain[:len(model.ApiTokenPrefix)+8],
	}
	if in.ExpireDays > 0 {
		exp := time.Now().AddDate(0, 0, in.ExpireDays)
		t.ExpireAt = &exp
	}
	if err := s.tokens.Create(t); err != nil {
		return nil, err
	}
	return &CreateResult{Token: t, Plain: plain}, nil
}

// List 当前用户的令牌列表。
func (s *TokenService) List(userID uint) ([]model.ApiToken, error) {
	return s.tokens.ListByUser(userID)
}

// Revoke 吊销令牌（仅限属主）。
func (s *TokenService) Revoke(id, userID uint) error {
	return s.tokens.Delete(id, userID)
}

// Resolve 鉴权中间件回调：明文 token → 用户三要素。
// 失败统一返回 ok=false（原因不透出给调用方）。
func (s *TokenService) Resolve(plain string) (userID uint, username, role string, ok bool) {
	if !strings.HasPrefix(plain, model.ApiTokenPrefix) {
		return 0, "", "", false
	}
	sum := sha256.Sum256([]byte(plain))
	t, err := s.tokens.FindValidByHash(hex.EncodeToString(sum[:]))
	if err != nil {
		return 0, "", "", false
	}
	u, err := s.users.FindByID(t.UserID)
	if err != nil || u.Status != 1 {
		return 0, "", "", false
	}
	go s.tokens.Touch(t.ID)
	return u.ID, u.Username, u.Role, true
}
