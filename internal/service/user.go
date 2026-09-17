package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/repo"
)

var (
	ErrUsernameTaken    = errors.New("用户名已存在")
	ErrCannotDeleteSelf = errors.New("不能删除自己的账号")
	ErrLastAdmin        = errors.New("不能停用或降级最后一个管理员")
	ErrInvalidRole      = errors.New("非法角色（可选 admin / operator / viewer）")
	ErrUserNotFound     = errors.New("用户不存在")
)

// UserService 用户管理与 Zone 授权。
type UserService struct {
	users  *repo.UserRepo
	grants *repo.GrantRepo
	audit  *repo.AuditRepo
}

func NewUserService(users *repo.UserRepo, grants *repo.GrantRepo, audit *repo.AuditRepo) *UserService {
	return &UserService{users: users, grants: grants, audit: audit}
}

func validRole(r string) bool {
	return r == model.RoleAdmin || r == model.RoleOperator || r == model.RoleViewer
}

// List 用户列表。
func (s *UserService) List() ([]model.User, error) {
	return s.users.List()
}

// Create 创建用户。
func (s *UserService) Create(username, password, role string, opUserID uint, opUsername string) (*model.User, error) {
	if !validRole(role) {
		return nil, ErrInvalidRole
	}
	if len(username) < 2 || len(password) < 6 {
		return nil, fmt.Errorf("用户名至少 2 位，密码至少 6 位")
	}
	if existing, _ := s.users.FindByUsername(username); existing != nil {
		return nil, ErrUsernameTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{Username: username, PasswordHash: string(hash), Role: role, Status: 1}
	if err := s.users.Create(u); err != nil {
		return nil, err
	}
	s.writeAudit(opUserID, opUsername, "user.create", username, map[string]string{"role": role}, nil)
	return u, nil
}

// Update 更新用户：角色/状态/重置密码（字段留空表示不改）。
func (s *UserService) Update(id uint, role string, status *int, newPassword string, opUserID uint, opUsername string) error {
	u, err := s.users.FindByID(id)
	if err != nil || u == nil {
		return ErrUserNotFound
	}
	if role != "" {
		if !validRole(role) {
			return ErrInvalidRole
		}
		// 降级/停用最后一个 admin 的防护
		if u.Role == model.RoleAdmin && (role != model.RoleAdmin || (status != nil && *status != 1)) {
			if err := s.guardLastAdmin(u); err != nil {
				return err
			}
		}
		u.Role = role
	}
	if status != nil {
		if *status != 1 && u.Role == model.RoleAdmin {
			if err := s.guardLastAdmin(u); err != nil {
				return err
			}
		}
		u.Status = *status
	}
	if newPassword != "" {
		if len(newPassword) < 6 {
			return fmt.Errorf("密码至少 6 位")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u.PasswordHash = string(hash)
	}
	if err := s.users.Update(u); err != nil {
		return err
	}
	detail := map[string]any{"role": u.Role, "status": u.Status, "password_reset": newPassword != ""}
	s.writeAudit(opUserID, opUsername, "user.update", u.Username, detail, nil)
	return nil
}

// Delete 删除用户。
func (s *UserService) Delete(id, opUserID uint, opUsername string) error {
	if id == opUserID {
		return ErrCannotDeleteSelf
	}
	u, err := s.users.FindByID(id)
	if err != nil || u == nil {
		return ErrUserNotFound
	}
	if u.Role == model.RoleAdmin {
		if err := s.guardLastAdmin(u); err != nil {
			return err
		}
	}
	if err := s.users.Delete(id); err != nil {
		return err
	}
	s.writeAudit(opUserID, opUsername, "user.delete", u.Username, nil, nil)
	return nil
}

func (s *UserService) guardLastAdmin(u *model.User) error {
	n, err := s.users.CountAdmins()
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// --- Zone 授权 ---

// Grants 查询用户的 Zone 授权。
func (s *UserService) Grants(userID uint) ([]model.UserZone, error) {
	return s.grants.ZonesByUser(userID)
}

// SetGrants 全量设置用户的 Zone 授权（account_id + zone 组合）。
func (s *UserService) SetGrants(userID uint, items []model.UserZone, opUserID uint, opUsername string) error {
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return ErrUserNotFound
	}
	if u.Role == model.RoleAdmin {
		return fmt.Errorf("admin 用户拥有全部权限，无需授权")
	}
	for i := range items {
		items[i].UserID = userID
		items[i].ID = 0
	}
	if err := s.grants.ReplaceByUser(userID, items); err != nil {
		return err
	}
	zones := make([]string, 0, len(items))
	for _, it := range items {
		zones = append(zones, fmt.Sprintf("%d:%s", it.CloudAccountID, it.Zone))
	}
	s.writeAudit(opUserID, opUsername, "user.grant", u.Username, map[string]any{"zones": zones}, nil)
	return nil
}

// AccessibleZones 非 admin 用户在指定账号下可访问的 Zone 集合。
func (s *UserService) AccessibleZones(userID, accountID uint) (map[string]bool, error) {
	return s.grants.ZonesByUserAndAccount(userID, accountID)
}

// HasZoneAccess 判断用户是否有某 账号+Zone 的权限（admin 直通）。
func (s *UserService) HasZoneAccess(userID uint, role string, accountID uint, zone string) (bool, error) {
	if role == model.RoleAdmin {
		return true, nil
	}
	return s.grants.Exists(userID, accountID, zone)
}

// Audit2FAReset 记录管理员重置用户两步验证的审计日志。
func (s *UserService) Audit2FAReset(opUserID uint, opUsername string, targetUserID uint) {
	s.writeAudit(opUserID, opUsername, "user.reset_2fa", fmt.Sprintf("user#%d", targetUserID),
		map[string]any{"target_user_id": targetUserID}, nil)
}

// writeAudit 用户操作审计。
func (s *UserService) writeAudit(userID uint, username, action, resource string, detail any, err error) {
	detailJSON, _ := json.Marshal(detail)
	log := model.AuditLog{
		UserID: userID, Username: username, Action: action,
		Resource: resource, Detail: string(detailJSON), Status: "success",
	}
	if err != nil {
		log.Status = "failed"
		log.Message = err.Error()
	}
	_ = s.audit.Create(&log)
}
