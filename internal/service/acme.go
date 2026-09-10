package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/cryptox"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/provider"
	"github.com/domhub-io/domhub/internal/repo"
)

// ACME CA 目录注册表（证书申请一期：免费证书自动签发）。
var acmeDirectories = map[string]struct {
	URL  string
	Name string
}{
	"letsencrypt":         {URL: "https://acme-v02.api.letsencrypt.org/directory", Name: "Let's Encrypt"},
	"letsencrypt_staging": {URL: "https://acme-staging-v02.api.letsencrypt.org/directory", Name: "Let's Encrypt (测试环境)"},
	"zerossl":             {URL: "https://acme.zerossl.com/v2/DV90", Name: "ZeroSSL"},
}

// AcmeCAS 供前端枚举可用的 CA。
type AcmeCAS struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func SupportedCAs() []AcmeCAS {
	keys := make([]string, 0, len(acmeDirectories))
	for k := range acmeDirectories {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]AcmeCAS, 0, len(keys))
	for _, k := range keys {
		d := acmeDirectories[k]
		out = append(out, AcmeCAS{Key: k, Name: d.Name, URL: d.URL})
	}
	return out
}

// AcmeService 证书申请：ACME + DNS-01（复用已接入云账号的解析通道）。
type AcmeService struct {
	accounts    *repo.AcmeAccountRepo
	issued      *repo.IssuedCertRepo
	zoneRepo    *repo.ZoneRepo
	dns         *DNSService
	accountRepo *repo.CloudAccountRepo
	cipher      *cryptox.Cipher

	mu sync.Mutex // 串行化 ACME 流程，避免同账户并发注册/触发 CA 限流
}

func NewAcmeService(
	accounts *repo.AcmeAccountRepo,
	issued *repo.IssuedCertRepo,
	zoneRepo *repo.ZoneRepo,
	dns *DNSService,
	accountRepo *repo.CloudAccountRepo,
	cipher *cryptox.Cipher,
) *AcmeService {
	return &AcmeService{
		accounts: accounts, issued: issued, zoneRepo: zoneRepo,
		dns: dns, accountRepo: accountRepo, cipher: cipher,
	}
}

// Create 创建申请并异步执行：先落库（pending）可查询，后台跑 ACME 流程。
func (s *AcmeService) Create(dnsAccountID uint, domains []string, email, ca string, autoRenew bool) (*model.IssuedCert, error) {
	dir, ok := acmeDirectories[ca]
	if !ok {
		return nil, fmt.Errorf("不支持的证书 CA: %s", ca)
	}
	domains = normalizeDomains(domains)
	if len(domains) == 0 {
		return nil, errors.New("至少需要一个有效域名")
	}
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("联系邮箱无效")
	}
	if _, err := s.accountRepo.FindByID(dnsAccountID); err != nil {
		return nil, errors.New("DNS 云账号不存在")
	}
	// 账号必须真的管着主域名的解析（Zone 缓存中按后缀匹配）
	if _, err := s.resolveZone(dnsAccountID, domains[0]); err != nil {
		return nil, err
	}

	c := &model.IssuedCert{
		PrimaryDomain: domains[0],
		SANs:          strings.Join(domains, ","),
		DNSAccountID:  dnsAccountID,
		AcmeEmail:     email,
		DirectoryURL:  dir.URL,
		CAName:        dir.Name,
		Status:        model.CertApplyPending,
		AutoRenew:     autoRenew,
		ProgressLog:   nowStamp() + " 申请已创建，等待执行",
	}
	if err := s.issued.Create(c); err != nil {
		return nil, err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.runRenew(ctx, c, domains)
	}()
	return c, nil
}

// RenewNow 手动立即续期（沿用原申请参数）。
func (s *AcmeService) RenewNow(id uint) error {
	c, err := s.issued.FindByID(id)
	if err != nil {
		return err
	}
	if c.Status != model.CertApplyIssued && c.Status != model.CertApplyFailed {
		return fmt.Errorf("当前状态 %s 不允许续期", c.Status)
	}
	domains := strings.Split(c.SANs, ",")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		// 重新读取最新记录再执行
		latest, err := s.issued.FindByID(id)
		if err != nil {
			return
		}
		s.runRenew(ctx, latest, domains)
	}()
	return nil
}

// RunRenewals 自动续期任务（cert_renew 定时调用）：withinDays 天内到期的已签发证书逐个续期。
func (s *AcmeService) RunRenewals(withinDays int) (renewed, failed int) {
	list, err := s.issued.DueForRenewal(withinDays)
	if err != nil {
		logger.L().Error("读取待续期证书失败", zap.Error(err))
		return 0, 0
	}
	for i := range list {
		c := &list[i]
		domains := strings.Split(c.SANs, ",")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		s.mu.Lock()
		err := s.runRenewSync(ctx, c, domains)
		s.mu.Unlock()
		cancel()
		if err != nil {
			failed++
		} else {
			renewed++
		}
		time.Sleep(2 * time.Second) // 对 CA 的礼貌间隔
	}
	return renewed, failed
}

// ---- 申请/续期共用流程 ----

// runRenew 异步入口（panic 兜底后转为失败状态）。
func (s *AcmeService) runRenew(ctx context.Context, c *model.IssuedCert, domains []string) {
	defer func() {
		if r := recover(); r != nil {
			logger.L().Error("ACME 流程异常", zap.Any("recover", r), zap.Uint("cert_id", c.ID))
		}
	}()
	_ = s.runRenewSync(ctx, c, domains)
}

// runRenewSync 同步执行完整 ACME 流程，返回 error 供自动续期统计。
func (s *AcmeService) runRenewSync(ctx context.Context, c *model.IssuedCert, domains []string) error {
	c.Status = model.CertApplyRenewing
	if c.ID != 0 {
		_ = s.issued.Update(c)
	}
	log := func(format string, args ...any) {
		c.ProgressLog = strings.TrimSpace(c.ProgressLog + "\n" + nowStamp() + " " + fmt.Sprintf(format, args...))
		_ = s.issued.Update(c)
	}
	fail := func(format string, args ...any) error {
		err := fmt.Errorf(format, args...)
		c.Status = model.CertApplyFailed
		c.LastMessage = err.Error()
		log("❌ %s", err.Error())
		_ = s.issued.Update(c)
		return err
	}

	// 1. ACME 账户（按 CA+邮箱复用）
	key, err := s.ensureAccount(ctx, c)
	if err != nil {
		return fail("ACME 账户注册失败: %v", err)
	}
	client := &acme.Client{Key: key, DirectoryURL: c.DirectoryURL}

	// 2. 下单
	idents := make([]acme.AuthzID, 0, len(domains))
	for _, d := range domains {
		idents = append(idents, acme.AuthzID{Type: "dns", Value: d})
	}
	order, err := client.AuthorizeOrder(ctx, idents)
	if err != nil {
		return fail("创建证书订单失败: %v", err)
	}
	log("✅ 订单已创建，共 %d 个域名待验证", len(domains))

	// 3. 逐域名 DNS-01 验证
	if err := s.challengeAll(ctx, client, order, c, log); err != nil {
		return fail("DNS-01 验证失败: %v", err)
	}

	// 4. 等订单就绪并签发
	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return fail("等待订单就绪失败: %v", err)
	}
	log("✅ 域名验证全部通过，开始签发")

	certPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fail("生成证书私钥失败: %v", err)
	}
	tpl := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domains[0]},
		DNSNames: domains,
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tpl, certPriv)
	if err != nil {
		return fail("生成 CSR 失败: %v", err)
	}
	// 注意：CreateOrderCert 的第一个参数必须是 FinalizeURL（订单 URI 只允许 POST-as-GET）
	derChain, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return fail("签发失败: %v", err)
	}

	// 5. 解析链与有效期，落库
	var chainPEM strings.Builder
	for _, der := range derChain {
		chainPEM.WriteString(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	}
	leaf, err := x509.ParseCertificate(derChain[0])
	if err != nil {
		return fail("证书解析失败: %v", err)
	}
	keyPEM, err := x509.MarshalECPrivateKey(certPriv)
	if err != nil {
		return fail("私钥序列化失败: %v", err)
	}
	privEnc, err := s.cipher.Encrypt(string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyPEM})))
	if err != nil {
		return fail("私钥加密失败: %v", err)
	}

	isRenew := c.NotAfter != nil
	c.Status = model.CertApplyIssued
	c.CertChain = chainPEM.String()
	c.PrivateKeyEnc = privEnc
	c.NotBefore = &leaf.NotBefore
	c.NotAfter = &leaf.NotAfter
	c.LastMessage = ""
	c.RenewCount++
	if isRenew {
		log("🎉 续期成功，新证书有效期至 %s", leaf.NotAfter.Format("2006-01-02"))
	} else {
		log("🎉 签发成功，有效期至 %s（CA: %s）", leaf.NotAfter.Format("2006-01-02"), c.CAName)
	}
	return s.issued.Update(c)
}

// challengeAll 对订单内全部授权执行 DNS-01（含 TXT 清理兜底）。
func (s *AcmeService) challengeAll(ctx context.Context, client *acme.Client, order *acme.Order, c *model.IssuedCert, log func(string, ...any)) error {
	type txtRef struct {
		zone, recordID string
	}
	var created []txtRef
	defer func() {
		// 无论成败都清理验证记录
		for _, t := range created {
			_ = s.dns.DeleteRecord(c.DNSAccountID, t.zone, t.recordID, "ACME DNS-01 验证记录", systemActor())
		}
		if len(created) > 0 {
			log("🧹 已清理 %d 条 _acme-challenge TXT 验证记录", len(created))
		}
	}()

	for _, authzURL := range order.AuthzURLs {
		az, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("拉取授权失败: %w", err)
		}
		if az.Status == acme.StatusValid {
			continue
		}
		domain := az.Identifier.Value
		var chal *acme.Challenge
		for _, ch := range az.Challenges {
			if ch.Type == "dns-01" {
				chal = ch
				break
			}
		}
		if chal == nil {
			return fmt.Errorf("%s 的授权中没有 dns-01 验证方式", domain)
		}
		txtValue, err := client.DNS01ChallengeRecord(chal.Token)
		if err != nil {
			return fmt.Errorf("计算验证记录失败: %w", err)
		}

		// 定位该域名归属的 Zone（按后缀最长匹配）
		zone, err := s.resolveZone(c.DNSAccountID, domain)
		if err != nil {
			return err
		}
		// TXT 的 FQDN 按定义就是 _acme-challenge.<验证域名>（泛域名时 CA 的 Identifier 已是主域）
		txtFQDN := "_acme-challenge." + strings.TrimSuffix(strings.ToLower(domain), ".")
		// 相对记录名（写入 Zone 时用）：去掉 Zone 后缀
		recordName := txtFQDN
		if z := strings.ToLower(strings.TrimSuffix(zone, ".")); strings.HasSuffix(txtFQDN, "."+z) {
			recordName = strings.TrimSuffix(txtFQDN, "."+z)
		}
		recordID, err := s.dns.CreateRecord(
			c.DNSAccountID, zone,
			provider.RecordInfo{Name: recordName, Type: "TXT", Value: txtValue, TTL: 600},
			systemActor(),
		)
		if err != nil {
			return fmt.Errorf("写入验证 TXT 记录失败（%s @ %s）: %w", recordName, zone, err)
		}
		created = append(created, txtRef{zone: zone, recordID: recordID})
		log("📝 已写入验证记录 %s → %s…（%s）", recordName, txtValue[:12], zone)

		// 等待 TXT 在权威 NS 上生效（LE 直查权威；递归解析器有缓存会造成假阳性），
		// 全部权威确认后才触发验证，轮询最多 5 分钟
		if err := waitTXTAuthoritative(ctx, zone, txtFQDN, txtValue); err != nil {
			return fmt.Errorf("验证记录生效检查失败: %w", err)
		}
		log("✅ 验证记录已在全部权威 NS 生效（%s）", txtFQDN)

		if _, err := client.Accept(ctx, chal); err != nil {
			return fmt.Errorf("提交验证失败（%s）: %w", domain, err)
		}
		if err := s.waitAuthz(ctx, client, authzURL); err != nil {
			return err
		}
		log("✅ %s 域名验证通过", domain)
	}
	return nil
}

func (s *AcmeService) waitAuthz(ctx context.Context, client *acme.Client, authzURL string) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		az, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("轮询授权状态失败: %w", err)
		}
		switch az.Status {
		case acme.StatusValid:
			return nil
		case acme.StatusInvalid:
			for _, ch := range az.Challenges {
				if ch.Error != nil {
					var acmeErr *acme.Error
					if errors.As(ch.Error, &acmeErr) && acmeErr.Detail != "" {
						return fmt.Errorf("验证被 CA 拒绝: %s", acmeErr.Detail)
					}
					return fmt.Errorf("验证被 CA 拒绝: %v", ch.Error)
				}
			}
			return errors.New("验证被 CA 拒绝")
		}
		if time.Now().After(deadline) {
			return errors.New("验证超时（2 分钟内未完成）")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// waitTXTAuthoritative 轮询 Zone 的全部权威 NS，直至每个 NS 都返回预期 TXT 值。
// 直查权威而非递归解析器：递归的负缓存/旧值缓存会误导校验时机（CA 直查权威）。
func waitTXTAuthoritative(ctx context.Context, zone, fqdn, expect string) error {
	zone = strings.TrimSuffix(strings.ToLower(zone), ".")
	deadline := time.Now().Add(5 * time.Minute)
	var nameservers []string
	for {
		if len(nameservers) == 0 {
			if nss, err := net.LookupNS(zone); err == nil && len(nss) > 0 {
				for _, ns := range nss {
					nameservers = append(nameservers, strings.TrimSuffix(ns.Host, "."))
				}
			}
		}
		if len(nameservers) == 0 {
			return fmt.Errorf("无法解析 %s 的权威 NS", zone)
		}
		allOK := true
		for _, ns := range nameservers {
			ok, err := queryTXTOnNS(ctx, ns, fqdn, expect)
			if err != nil || !ok {
				allOK = false
				break
			}
		}
		if allOK {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("5 分钟内 TXT 未在全部权威 NS 生效（%s @ %s）", fqdn, strings.Join(nameservers, ","))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// queryTXTOnNS 直接向指定权威 NS 发起 TXT 查询（绕过递归缓存）。
func queryTXTOnNS(ctx context.Context, nsHost, fqdn, expect string) (bool, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, net.JoinHostPort(nsHost, "53"))
		},
	}
	values, err := r.LookupTXT(ctx, fqdn)
	if err != nil {
		return false, err
	}
	for _, v := range values {
		if v == expect {
			return true, nil
		}
	}
	return false, nil
}

// ensureAccount 获取或注册 ACME 账户（邮箱取证书记录的 AcmeEmail，续期沿用）。
func (s *AcmeService) ensureAccount(ctx context.Context, c *model.IssuedCert) (*ecdsa.PrivateKey, error) {
	existing, err := s.accounts.FindByDirAndEmail(c.DirectoryURL, c.AcmeEmail)
	if err == nil {
		return s.decryptAccountKey(existing.KeyEnc)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if c.AcmeEmail == "" {
		return nil, errors.New("缺少联系邮箱，无法注册 ACME 账户")
	}

	accKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	client := &acme.Client{Key: accKey, DirectoryURL: c.DirectoryURL}
	reg, err := client.Register(ctx, &acme.Account{Contact: []string{"mailto:" + c.AcmeEmail}}, func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	keyPEM, err := x509.MarshalECPrivateKey(accKey)
	if err != nil {
		return nil, err
	}
	enc, err := s.cipher.Encrypt(string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyPEM})))
	if err != nil {
		return nil, err
	}
	acc := &model.AcmeAccount{
		DirectoryURL: c.DirectoryURL, Email: c.AcmeEmail,
		KeyEnc: enc, RegistrationURI: reg.URI,
	}
	if err := s.accounts.Create(acc); err != nil {
		return nil, err
	}
	logger.L().Info("已注册 ACME 账户", zap.String("ca", c.CAName), zap.String("email", c.AcmeEmail))
	return accKey, nil
}

func (s *AcmeService) decryptAccountKey(enc string) (*ecdsa.PrivateKey, error) {
	plain, err := s.cipher.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(plain))
	if block == nil {
		return nil, errors.New("账户私钥 PEM 解析失败")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// resolveZone 在指定账号的 Zone 缓存里按后缀最长匹配定位 domain 的托管 Zone。
func (s *AcmeService) resolveZone(accountID uint, domain string) (string, error) {
	views, err := s.zoneRepo.ListViews()
	if err != nil {
		return "", err
	}
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	best := ""
	for _, v := range views {
		if v.CloudAccountID != accountID {
			continue
		}
		z := strings.ToLower(strings.TrimSuffix(v.Name, "."))
		if domain == z || strings.HasSuffix(domain, "."+z) {
			if len(z) > len(best) {
				best = z
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("该云账号下未找到能承载 %s 解析的 Zone，请确认域名解析已托管到该账号", domain)
	}
	return best, nil
}

func normalizeDomains(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimSuffix(d, ".")
		if d == "" || seen[d] || !validDomain(d) {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func validDomain(d string) bool {
	labels := strings.Split(strings.TrimPrefix(d, "*."), ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if l == "" || len(l) > 63 {
			return false
		}
		for _, r := range l {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
			if !ok {
				return false
			}
		}
	}
	return true
}

func nowStamp() string { return time.Now().Format("01-02 15:04:05") }

// ---- 供 handler 使用的导出方法 ----

// List 证书列表。
func (s *AcmeService) List(status string) ([]model.IssuedCert, error) { return s.issued.List(status) }

// Get 单条详情。
func (s *AcmeService) Get(id uint) (*model.IssuedCert, error) { return s.issued.FindByID(id) }

// Delete 删除证书记录（不撤销 CA 侧证书）。
func (s *AcmeService) Delete(id uint) error { return s.issued.Delete(id) }

// ExportPEM 导出证书链/私钥/合并 fullchain（私钥解密后返回）。
func (s *AcmeService) ExportPEM(id uint, kind string) (string, error) {
	c, err := s.issued.FindByID(id)
	if err != nil {
		return "", err
	}
	switch kind {
	case "chain":
		if c.CertChain == "" {
			return "", errors.New("证书尚未签发")
		}
		return c.CertChain, nil
	case "key":
		if c.PrivateKeyEnc == "" {
			return "", errors.New("证书尚未签发")
		}
		return s.cipher.Decrypt(c.PrivateKeyEnc)
	case "fullchain":
		chain, err := s.ExportPEM(id, "chain")
		if err != nil {
			return "", err
		}
		key, err := s.ExportPEM(id, "key")
		if err != nil {
			return "", err
		}
		return chain + "\n" + key, nil
	default:
		return "", fmt.Errorf("不支持的导出类型: %s", kind)
	}
}

// SetAutoRenew 开关自动续期。
func (s *AcmeService) SetAutoRenew(id uint, on bool) error {
	c, err := s.issued.FindByID(id)
	if err != nil {
		return err
	}
	c.AutoRenew = on
	return s.issued.Update(c)
}

// systemActor ACME 流程为系统行为（审计与授权按 admin 直通）。
func systemActor() Actor { return Actor{ID: 0, Username: "acme", Role: model.RoleAdmin} }
