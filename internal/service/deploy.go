package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/pkg/cryptox"
	"github.com/onemore-coder/domhub/internal/pkg/logger"
	"github.com/onemore-coder/domhub/internal/pkg/notify"
	"github.com/onemore-coder/domhub/internal/provider"
	"github.com/onemore-coder/domhub/internal/provider/aliyun"
	"github.com/onemore-coder/domhub/internal/provider/tencent"
	"github.com/onemore-coder/domhub/internal/repo"
)

// CertDeployService 证书部署：签发/续期成功后把证书下发到 CDN 或 SSH 主机。
//
// 目标类型：
//   - aliyun_cdn / tencent_cdn：复用云账号 AK，上传证书并绑定到 CDN 域名
//   - ssh_host：SSH 登录主机写入证书/私钥并执行 reload 命令
type CertDeployService struct {
	deploys  *repo.CertDeployRepo
	issued   *repo.IssuedCertRepo
	accounts *repo.CloudAccountRepo
	audit    *repo.AuditRepo
	cipher   *cryptox.Cipher
	rules    *repo.AlertRepo // 部署失败通知
}

func NewCertDeployService(
	deploys *repo.CertDeployRepo,
	issued *repo.IssuedCertRepo,
	accounts *repo.CloudAccountRepo,
	audit *repo.AuditRepo,
	cipher *cryptox.Cipher,
	rules *repo.AlertRepo,
) *CertDeployService {
	return &CertDeployService{
		deploys: deploys, issued: issued, accounts: accounts,
		audit: audit, cipher: cipher, rules: rules,
	}
}

// ---- 配置结构 ----

type cdnConfig struct {
	Domain string `json:"domain"`
}

type sshConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	CertPath  string `json:"cert_path"`
	KeyPath   string `json:"key_path"`
	ReloadCmd string `json:"reload_cmd"`
}

type sshSecret struct {
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

// ---- CRUD ----

// List 列出某证书的全部部署目标。
func (s *CertDeployService) List(certID uint) ([]model.CertDeploy, error) {
	return s.deploys.ListByCert(certID)
}

// SaveTarget 创建/更新部署目标（id>0 为更新，certID 可由已有记录回填）。
func (s *CertDeployService) SaveTarget(id, certID uint, typ, name string, accountID uint, config map[string]any, secret map[string]any) (*model.CertDeploy, error) {
	if id > 0 {
		prev, err := s.deploys.FindByID(id)
		if err != nil {
			return nil, fmt.Errorf("部署目标不存在")
		}
		if certID == 0 {
			certID = prev.CertID
		}
		if prev.CertID != certID {
			return nil, fmt.Errorf("部署目标与证书不匹配")
		}
	} else if _, err := s.issued.FindByID(certID); err != nil {
		return nil, fmt.Errorf("证书不存在")
	}
	switch typ {
	case model.DeployAliyunCDN, model.DeployTencentCDN:
		if accountID == 0 {
			return nil, fmt.Errorf("请选择 CDN 所在的云账号")
		}
		if _, err := s.accounts.FindByID(accountID); err != nil {
			return nil, fmt.Errorf("云账号不存在")
		}
	case model.DeploySSHHost:
		var cfg sshConfig
		if err := remap(config, &cfg); err != nil {
			return nil, fmt.Errorf("SSH 配置格式错误")
		}
		if cfg.Host == "" || cfg.User == "" || cfg.CertPath == "" || cfg.KeyPath == "" {
			return nil, fmt.Errorf("SSH 主机/用户/证书路径/私钥路径均为必填")
		}
	default:
		return nil, fmt.Errorf("不支持的部署类型: %s", typ)
	}

	cfgJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	d := &model.CertDeploy{
		ID: id, CertID: certID, Type: typ, Name: name,
		AccountID: accountID, Config: string(cfgJSON),
	}
	if id > 0 {
		prev, err := s.deploys.FindByID(id)
		if err != nil {
			return nil, fmt.Errorf("部署目标不存在")
		}
		if certID == 0 {
			certID = prev.CertID
		}
		if prev.CertID != certID {
			return nil, fmt.Errorf("部署目标与证书不匹配")
		}
		d = prev
		d.Type, d.Name, d.AccountID, d.Config = typ, name, accountID, string(cfgJSON)
	}
	// SSH 敏感字段：仅当本次传了才覆盖（编辑时留空表示沿用旧值）
	if typ == model.DeploySSHHost {
		var sec sshSecret
		if secJSON, err := json.Marshal(secret); err == nil {
			_ = json.Unmarshal(secJSON, &sec)
		}
		if sec.Password != "" || sec.PrivateKey != "" {
			enc, err := s.cipher.Encrypt(mustJSON(sec))
			if err != nil {
				return nil, fmt.Errorf("敏感配置加密失败: %w", err)
			}
			d.SecretEnc = enc
		} else if d.ID == 0 {
			return nil, fmt.Errorf("SSH 登录凭证未配置（密码或私钥至少填一项）")
		}
	}
	if d.ID > 0 {
		return d, s.deploys.Update(d)
	}
	return d, s.deploys.Create(d)
}

// Delete 删除部署目标。
func (s *CertDeployService) Delete(id uint) error { return s.deploys.Delete(id) }

// ---- 执行 ----

// Run 手动执行单个部署目标。
func (s *CertDeployService) Run(id uint, op Actor) error {
	d, err := s.deploys.FindByID(id)
	if err != nil {
		return fmt.Errorf("部署目标不存在")
	}
	msg := s.execute(d)
	s.writeAudit(op.ID, op.Username, "cert.deploy", fmt.Sprintf("cert#%d/%s", d.CertID, d.Type), map[string]any{
		"deploy_id": d.ID, "name": d.Name,
	}, strErr(msg))
	return nil // 结果落在目标状态上，审计不阻断
}

// RunForCert 执行某证书的全部部署目标（签发/续期成功后的钩子，异步调用）。
func (s *CertDeployService) RunForCert(certID uint) {
	targets, err := s.deploys.ListByCert(certID)
	if err != nil || len(targets) == 0 {
		return
	}
	ok, failed := 0, 0
	for i := range targets {
		if s.execute(&targets[i]) == "" {
			ok++
		} else {
			failed++
		}
		time.Sleep(time.Second) // 对厂商 API 温和限速
	}
	logger.L().Info("证书部署完成", zap.Uint("cert_id", certID),
		zap.Int("success", ok), zap.Int("failed", failed))
}

// execute 执行单个目标；成功返回 ""，失败返回错误信息。
func (s *CertDeployService) execute(d *model.CertDeploy) string {
	now := time.Now()
	errMsg := s.doExecute(d)

	d.LastDeployedAt = &now
	d.LastMessage = errMsg
	if errMsg == "" {
		d.Status = model.DeploySuccess
		d.LastMessage = "部署成功"
	} else {
		d.Status = model.DeployFailed
		if len(d.LastMessage) > 500 {
			d.LastMessage = d.LastMessage[:500]
		}
	}
	if err := s.deploys.Update(d); err != nil {
		logger.L().Error("更新部署目标状态失败", zap.Uint("deploy_id", d.ID), zap.Error(err))
	}

	if errMsg != "" {
		logger.L().Error("证书部署失败", zap.Uint("deploy_id", d.ID), zap.String("error", errMsg))
		s.notifyFailure(d, errMsg)
	}
	return errMsg
}

func (s *CertDeployService) doExecute(d *model.CertDeploy) string {
	c, err := s.issued.FindByID(d.CertID)
	if err != nil {
		return "证书不存在"
	}
	if c.Status != model.CertApplyIssued || c.CertChain == "" || c.PrivateKeyEnc == "" {
		return "证书尚未签发，无法部署"
	}
	keyPEM, err := s.cipher.Decrypt(c.PrivateKeyEnc)
	if err != nil {
		return fmt.Sprintf("私钥解密失败: %v", err)
	}
	certName := fmt.Sprintf("domhub-%s-%d", strings.ReplaceAll(c.PrimaryDomain, ".", "-"), time.Now().Unix())

	switch d.Type {
	case model.DeployAliyunCDN:
		var cfg cdnConfig
		if err := json.Unmarshal([]byte(d.Config), &cfg); err != nil || cfg.Domain == "" {
			return "CDN 配置缺少 domain"
		}
		cred, err := s.accountCredential(d.AccountID)
		if err != nil {
			return err.Error()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := aliyun.NewCertDeployClient(*cred).SetCDNCert(ctx, cfg.Domain, certName, c.CertChain, keyPEM); err != nil {
			return err.Error()
		}
	case model.DeployTencentCDN:
		var cfg cdnConfig
		if err := json.Unmarshal([]byte(d.Config), &cfg); err != nil || cfg.Domain == "" {
			return "CDN 配置缺少 domain"
		}
		cred, err := s.accountCredential(d.AccountID)
		if err != nil {
			return err.Error()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := tencent.NewCertDeployClient(*cred).DeployCDNCert(ctx, cfg.Domain, certName, c.CertChain, keyPEM); err != nil {
			return err.Error()
		}
	case model.DeploySSHHost:
		if err := s.sshDeploy(d, c.CertChain, keyPEM); err != nil {
			return err.Error()
		}
	default:
		return fmt.Sprintf("不支持的部署类型: %s", d.Type)
	}
	return ""
}

// sshDeploy SSH 登录主机：写入证书/私钥 → 执行 reload 命令。
func (s *CertDeployService) sshDeploy(d *model.CertDeploy, chainPEM, keyPEM string) error {
	var cfg sshConfig
	if err := json.Unmarshal([]byte(d.Config), &cfg); err != nil {
		return fmt.Errorf("SSH 配置解析失败")
	}
	if d.SecretEnc == "" {
		return fmt.Errorf("SSH 登录凭证未配置（密码或私钥）")
	}
	plain, err := s.cipher.Decrypt(d.SecretEnc)
	if err != nil {
		return fmt.Errorf("SSH 凭证解密失败: %w", err)
	}
	var sec sshSecret
	if err := json.Unmarshal([]byte(plain), &sec); err != nil {
		return fmt.Errorf("SSH 凭证格式错误")
	}
	if cfg.Port <= 0 {
		cfg.Port = 22
	}

	auths := make([]ssh.AuthMethod, 0, 2)
	if sec.Password != "" {
		auths = append(auths, ssh.Password(sec.Password))
	}
	if sec.PrivateKey != "" {
		if signer, e := ssh.ParsePrivateKey([]byte(sec.PrivateKey)); e == nil {
			auths = append(auths, ssh.PublicKeys(signer))
		}
	}
	if len(auths) == 0 {
		return fmt.Errorf("无可用的 SSH 认证方式")
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // 自管主机场景，指纹管理后续迭代
		Timeout:         15 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	if err := sshWriteFile(client, cfg.CertPath, chainPEM, "644"); err != nil {
		return fmt.Errorf("写入证书失败: %w", err)
	}
	if err := sshWriteFile(client, cfg.KeyPath, keyPEM, "600"); err != nil {
		return fmt.Errorf("写入私钥失败: %w", err)
	}
	if cmd := strings.TrimSpace(cfg.ReloadCmd); cmd != "" {
		out, err := sshRun(client, cmd)
		if err != nil {
			return fmt.Errorf("reload 命令执行失败: %v, 输出: %s", err, truncateStr(out, 200))
		}
	}
	return nil
}

// sshWriteFile 通过 cat stdin 写远端文件并 chmod（PEM 内容走 stdin，避免命令行长度/转义问题）。
func sshWriteFile(client *ssh.Client, path, content, mode string) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdin = strings.NewReader(content)
	out, err := sess.CombinedOutput(fmt.Sprintf("cat > %s && chmod %s %s", shellQuote(path), mode, shellQuote(path)))
	if err != nil {
		return fmt.Errorf("%v, 输出: %s", err, truncateStr(string(out), 200))
	}
	return nil
}

// sshRun 执行远端命令。
func sshRun(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// accountCredential 解密云账号凭证。
func (s *CertDeployService) accountCredential(accountID uint) (*provider.Credential, error) {
	a, err := s.accounts.FindByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("云账号不存在")
	}
	ak, err := s.cipher.Decrypt(a.AccessKey)
	if err != nil {
		return nil, fmt.Errorf("AccessKey 解密失败: %w", err)
	}
	sk := ""
	if a.SecretKey != "" {
		if sk, err = s.cipher.Decrypt(a.SecretKey); err != nil {
			return nil, fmt.Errorf("SecretKey 解密失败: %w", err)
		}
	}
	return &provider.Credential{AccessKey: ak, SecretKey: sk, Region: a.Region}, nil
}

// notifyFailure 部署失败时通知所有启用渠道。
func (s *CertDeployService) notifyFailure(d *model.CertDeploy, errMsg string) {
	channels, err := s.rules.ListChannels()
	if err != nil {
		return
	}
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		n, err := notify.Build(ch.Type, ch.Config)
		if err != nil {
			continue
		}
		title := "【DomHub】证书部署失败"
		content := fmt.Sprintf("部署目标「%s」（%s）执行失败：\n%s\n请检查部署配置或手动重试。", d.Name, d.Type, errMsg)
		if err := n.Send(title, content); err != nil {
			logger.L().Warn("部署失败通知发送失败", zap.String("channel", ch.Name), zap.Error(err))
		}
	}
}

// ---- 小工具 ----

func remap(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func strErr(msg string) error {
	if msg == "" {
		return nil
	}
	return fmt.Errorf("%s", msg)
}

// writeAudit 部署审计（失败不阻断业务）。
func (s *CertDeployService) writeAudit(userID uint, username, action, resource string, detail any, err error) {
	detailJSON, _ := json.Marshal(detail)
	log := model.AuditLog{
		UserID: userID, Username: username, Action: action, Resource: resource,
		Detail: string(detailJSON), Status: "success",
	}
	if err != nil {
		log.Status = "failed"
		log.Message = err.Error()
	}
	if e := s.audit.Create(&log); e != nil {
		logger.L().Error("写入审计日志失败", zap.Error(e))
	}
}
