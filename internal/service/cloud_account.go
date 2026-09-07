// Package service 业务逻辑层。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/cryptox"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/provider"
	"github.com/domhub-io/domhub/internal/repo"
	"go.uber.org/zap"
)

// CloudAccountService 云账号业务。
type CloudAccountService struct {
	accounts *repo.CloudAccountRepo
	domains  *repo.DomainRepo
	tasks    *repo.SyncTaskRepo
	cipher   *cryptox.Cipher
}

func NewCloudAccountService(
	accounts *repo.CloudAccountRepo,
	domains *repo.DomainRepo,
	tasks *repo.SyncTaskRepo,
	cipher *cryptox.Cipher,
) *CloudAccountService {
	return &CloudAccountService{accounts: accounts, domains: domains, tasks: tasks, cipher: cipher}
}

// Create 接入云账号（凭证加密落库）。
func (s *CloudAccountService) Create(name, prov, ak, sk, region string) (*model.CloudAccount, error) {
	if _, err := provider.Get(prov); err != nil {
		return nil, err
	}
	if name == "" || ak == "" || sk == "" {
		return nil, fmt.Errorf("名称与凭证不能为空")
	}
	encAK, err := s.cipher.Encrypt(ak)
	if err != nil {
		return nil, fmt.Errorf("凭证加密失败: %w", err)
	}
	encSK, err := s.cipher.Encrypt(sk)
	if err != nil {
		return nil, fmt.Errorf("凭证加密失败: %w", err)
	}
	a := &model.CloudAccount{
		Name:      name,
		Provider:  prov,
		AccessKey: encAK,
		SecretKey: encSK,
		Region:    region,
		Status:    1,
	}
	if err := s.accounts.Create(a); err != nil {
		return nil, err
	}
	return a, nil
}

// List 账号列表（AK 脱敏）。
func (s *CloudAccountService) List() ([]model.CloudAccount, error) {
	list, err := s.accounts.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if ak, err := s.cipher.Decrypt(list[i].AccessKey); err == nil {
			list[i].AccessKey = cryptox.Mask(ak)
		} else {
			list[i].AccessKey = "****"
		}
	}
	return list, nil
}

// Update 更新账号（AK/SK 留空表示不修改）。
func (s *CloudAccountService) Update(id uint, name, region string, ak, sk string, status *int) error {
	a, err := s.accounts.FindByID(id)
	if err != nil {
		return err
	}
	if name != "" {
		a.Name = name
	}
	a.Region = region
	if status != nil {
		a.Status = *status
	}
	if ak != "" {
		enc, err := s.cipher.Encrypt(ak)
		if err != nil {
			return err
		}
		a.AccessKey = enc
	}
	if sk != "" {
		enc, err := s.cipher.Encrypt(sk)
		if err != nil {
			return err
		}
		a.SecretKey = enc
	}
	return s.accounts.Update(a)
}

// Delete 删除账号及其域名数据。
func (s *CloudAccountService) Delete(id uint) error {
	if err := s.domains.DeleteByAccount(id); err != nil {
		return err
	}
	return s.accounts.Delete(id)
}

// buildProvider 构建账号对应的云厂商 Provider。
func (s *CloudAccountService) buildProvider(a *model.CloudAccount) (provider.DomainProvider, error) {
	factory, err := provider.Get(a.Provider)
	if err != nil {
		return nil, err
	}
	ak, err := s.cipher.Decrypt(a.AccessKey)
	if err != nil {
		return nil, fmt.Errorf("AccessKey 解密失败: %w", err)
	}
	sk, err := s.cipher.Decrypt(a.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("SecretKey 解密失败: %w", err)
	}
	return factory(provider.Credential{AccessKey: ak, SecretKey: sk, Region: a.Region})
}

// Check 连通性检测。
func (s *CloudAccountService) Check(id uint) (*model.CloudAccount, error) {
	a, err := s.accounts.FindByID(id)
	if err != nil {
		return nil, err
	}
	p, err := s.buildProvider(a)
	if err != nil {
		s.recordCheck(a, false, err.Error())
		return a, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.CheckConnection(ctx); err != nil {
		s.recordCheck(a, false, err.Error())
		return a, nil
	}
	s.recordCheck(a, true, "连接成功")
	return a, nil
}

func (s *CloudAccountService) recordCheck(a *model.CloudAccount, ok bool, msg string) {
	now := time.Now()
	a.LastCheckAt = &now
	a.LastCheckOK = ok
	a.LastCheckMsg = msg
	if len(a.LastCheckMsg) > 500 {
		a.LastCheckMsg = a.LastCheckMsg[:500]
	}
	if err := s.accounts.Update(a); err != nil {
		logger.L().Error("保存检测结果失败", zap.Error(err))
	}
}

// Sync 同步单个账号的域名台账。
func (s *CloudAccountService) Sync(id uint) (*model.SyncTask, error) {
	a, err := s.accounts.FindByID(id)
	if err != nil {
		return nil, err
	}
	task := &model.SyncTask{
		CloudAccountID: a.ID,
		Type:           "sync",
		Status:         "running",
		StartedAt:      time.Now(),
	}
	if err := s.tasks.Create(task); err != nil {
		return nil, err
	}

	finish := func(status, msg string, count int) {
		task.Status = status
		task.Message = msg
		task.DomainCount = count
		now := time.Now()
		task.FinishedAt = &now
		if err := s.tasks.Finish(task); err != nil {
			logger.L().Error("保存同步任务失败", zap.Error(err))
		}
	}

	p, err := s.buildProvider(a)
	if err != nil {
		finish("failed", err.Error(), 0)
		return task, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	infos, err := p.ListDomains(ctx)
	if err != nil {
		finish("failed", err.Error(), 0)
		return task, nil
	}

	now := time.Now()
	items := make([]model.Domain, 0, len(infos))
	for _, info := range infos {
		item := model.Domain{
			CloudAccountID: a.ID,
			Name:           normalizeName(info.Name),
			Kind:           info.Kind,
			Provider:       a.Provider,
			Registrar:      info.Registrar,
			Status:         info.Status,
			LastSyncedAt:   now,
		}
		if !info.RegisteredAt.IsZero() {
			t := info.RegisteredAt
			item.RegisteredAt = &t
		}
		if !info.ExpireAt.IsZero() {
			t := info.ExpireAt
			item.ExpireAt = &t
		}
		items = append(items, item)
	}
	if err := s.domains.UpsertBatch(items); err != nil {
		finish("failed", "写入台账失败: "+err.Error(), 0)
		return task, nil
	}

	a.LastSyncAt = &now
	_ = s.accounts.Update(a)
	finish("success", "同步完成", len(items))
	return task, nil
}

// SyncAll 同步全部启用账号。
func (s *CloudAccountService) SyncAll() {
	list, err := s.accounts.List()
	if err != nil {
		return
	}
	for _, a := range list {
		if a.Status != 1 {
			continue
		}
		if _, err := s.Sync(a.ID); err != nil {
			logger.L().Error("同步账号失败", zap.String("name", a.Name), zap.Error(err))
		}
	}
}

// SupportedProviders 已支持的厂商列表。
func SupportedProviders() []string { return provider.Supported() }

// normalizeName 域名统一小写、去尾部点。
func normalizeName(n string) string { return strings.ToLower(strings.TrimSuffix(n, ".")) }
