package model

import "time"

// 证书部署目标类型。
const (
	DeployAliyunCDN  = "aliyun_cdn"  // 阿里云 CDN 域名绑定上传证书
	DeployTencentCDN = "tencent_cdn" // 腾讯云 CDN 域名绑定上传证书
	DeploySSHHost    = "ssh_host"    // SSH 推送到 Nginx 主机
)

// CertDeploy 证书部署目标（证书部署三期）。
// 一个证书可配置多个部署目标；签发/续期成功后自动执行全部目标（OnIssued 钩子），
// 也可手动触发。SSH 类型的敏感字段（密码/私钥）单独加密存储在 SecretEnc。
type CertDeploy struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	CertID uint   `gorm:"index;not null" json:"cert_id"` // 关联 issued_certs
	Type   string `gorm:"size:32;not null" json:"type"`  // aliyun_cdn | tencent_cdn | ssh_host
	Name   string `gorm:"size:128" json:"name"`          // 展示名
	// CDN 类型：目标厂商云账号（SSH 类型为 0）
	AccountID uint `gorm:"default:0" json:"account_id"`
	// 非敏感配置 JSON：
	//   CDN: {"domain":"cdn.example.com"}
	//   SSH: {"host":"1.2.3.4","port":22,"user":"root","cert_path":"/etc/nginx/ssl/site.crt","key_path":"/etc/nginx/ssl/site.key","reload_cmd":"nginx -s reload"}
	Config string `gorm:"type:text" json:"config"`
	// SSH 敏感配置 JSON（整体 AES-GCM 加密）：{"password":"...","private_key":"..."}
	SecretEnc string `gorm:"type:text" json:"-"`
	Status    string `gorm:"size:16;default:never" json:"status"` // never | success | failed
	LastMessage string     `gorm:"size:1023" json:"last_message"`
	LastDeployedAt *time.Time `json:"last_deployed_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (CertDeploy) TableName() string { return "cert_deploys" }

// CertDeployStatus 部署状态常量。
const (
	DeployNever   = "never"
	DeploySuccess = "success"
	DeployFailed  = "failed"
)
