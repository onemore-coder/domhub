package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/jwtx"
	"github.com/domhub-io/domhub/internal/repo"
)

// GitHubOAuthConf GitHub OAuth 应用配置。
type GitHubOAuthConf struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
}

// OAuthService GitHub OAuth 登录业务。
type OAuthService struct {
	conf    GitHubOAuthConf
	users   *repo.UserRepo
	db      *gorm.DB
	secret  string
	expireH int
}

func NewOAuthService(conf GitHubOAuthConf, users *repo.UserRepo, db *gorm.DB, jwtSecret string, expireHours int) *OAuthService {
	return &OAuthService{conf: conf, users: users, db: db, secret: jwtSecret, expireH: expireHours}
}

// Enabled 是否启用。
func (s *OAuthService) Enabled() bool {
	return s.conf.Enabled && s.conf.ClientID != "" && s.conf.ClientSecret != ""
}

// AuthURL 生成 GitHub 授权跳转地址（state 防 CSRF）。
func (s *OAuthService) AuthURL(redirectBase string) (string, string, error) {
	if !s.Enabled() {
		return "", "", errors.New("GitHub 登录未启用")
	}
	state := fmt.Sprintf("%d|%s", time.Now().UnixNano(), randomToken())
	// state 存服务端内存映射（单实例足够；多实例可换 cookie）
	stateStore.set(state, redirectBase, 10*time.Minute)
	u := "https://github.com/login/oauth/authorize?" + url.Values{
		"client_id": {s.conf.ClientID},
		"state":     {state},
		"scope":     {"read:user"},
	}.Encode()
	return u, state, nil
}

type githubUser struct {
	Login string `json:"login"`
	Email string `json:"email"`
}

// Exchange 用 code 换取 GitHub 用户并映射/创建本地用户，签发 DomHub JWT。
// 返回 (token, isNew, err)。
func (s *OAuthService) Exchange(ctx context.Context, code, state, redirectBase string) (string, bool, error) {
	if !stateStore.take(state) {
		return "", false, errors.New("state 无效或已过期，请重试")
	}

	// 1. code → access_token
	reqBody, _ := json.Marshal(map[string]string{
		"client_id":     s.conf.ClientID,
		"client_secret": s.conf.ClientSecret,
		"code":          code,
		"redirect_uri":  redirectBase + "/api/v1/auth/oauth/github/callback",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(string(reqBody)))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("请求 GitHub 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tokResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokResp); err != nil || tokResp.AccessToken == "" {
		return "", false, fmt.Errorf("GitHub 授权失败（code 无效或已使用）")
	}

	// 2. access_token → 用户信息
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	req2.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
	req2.Header.Set("Accept", "application/vnd.github+json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return "", false, fmt.Errorf("获取 GitHub 用户失败: %w", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	var ghUser githubUser
	if err := json.Unmarshal(body2, &ghUser); err != nil || ghUser.Login == "" {
		return "", false, fmt.Errorf("解析 GitHub 用户失败")
	}

	// 3. 映射本地用户：username = "gh:<login>"；首个 OAuth 用户为 admin，其余 viewer
	username := "gh:" + ghUser.Login
	existing, err := s.users.FindByUsername(username)
	isNew := false
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, err
		}
		role := model.RoleViewer
		var adminCount int64
		_ = s.db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&adminCount).Error
		if adminCount == 0 {
			role = model.RoleAdmin
		}
		tmpPwd, _ := bcrypt.GenerateFromPassword([]byte(randomToken()), bcrypt.DefaultCost)
		u := &model.User{Username: username, PasswordHash: string(tmpPwd), Role: role, Status: 1}
		if err := s.users.Create(u); err != nil {
			return "", false, err
		}
		existing = u
		isNew = true
	}
	if existing.Status != 1 {
		return "", false, errors.New("账号已被禁用")
	}

	token, err := jwtx.GenerateToken(existing.ID, existing.Username, existing.Role, s.secret, s.expireH)
	if err != nil {
		return "", false, err
	}
	return token, isNew, nil
}

// ---- state 内存存取（防 CSRF） ----

// randomToken 生成随机串（state / 临时密码用）。
func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

type stateEntry struct {
	createdAt time.Time
}

var stateStore = &oauthStateStore{items: map[string]stateEntry{}}

type oauthStateStore struct {
	items map[string]stateEntry
}

func (m *oauthStateStore) set(state, _ string, ttl time.Duration) {
	m.cleanup(ttl)
	m.items[state] = stateEntry{createdAt: time.Now()}
}

// take 校验并消费 state（一次性）。
func (m *oauthStateStore) take(state string) bool {
	e, ok := m.items[state]
	if !ok {
		return false
	}
	delete(m.items, state)
	return time.Since(e.createdAt) < 10*time.Minute
}

func (m *oauthStateStore) cleanup(ttl time.Duration) {
	for k, v := range m.items {
		if time.Since(v.createdAt) > ttl {
			delete(m.items, k)
		}
	}
}
