# Changelog

本项目的所有重要变更都记录在本文件中。

## [Unreleased]

### 变更

- **Docker 默认存储改为 SQLite**：`docker compose up -d` 即单容器部署，不再默认拉取 mysql:8.4（约 600MB），数据持久化到 `domhub_data` 卷（`/app/data/domhub.db`）；MySQL 场景改由叠加文件 `docker-compose.mysql.yml` 提供（`docker compose -f docker-compose.yml -f docker-compose.mysql.yml up -d`）
- **compose 显式分离双密钥**：新增 `DOMHUB_CRYPTO_KEY`（云凭证加密），与 `DOMHUB_JWT_SECRET` 独立，日后更换 JWT 密钥不影响已存凭证解密
- **Dockerfile**：预创建 `/app/data` 并归属运行用户（uid 10001），避免挂载卷后 SQLite 文件创建失败
- **鉴权网关兼容**（v0.7.1 后追加）：JWT 中间件多通道凭据（X-Api-Key → Authorization → domhub_token Cookie）+ 前端登录同步写 Cookie，适配会改写 Authorization 头的托管平台网关；新增 `cmd/demoseed` 演示环境自举工具

## [v0.7.1] - 2026-09-16

### 新增

- **两步验证（2FA / TOTP）**：零依赖 RFC 6238 实现（兼容 Google Authenticator 等验证器 App）；登录两段式（密码 → 6 位动态码），绑定走二维码扫码，关闭需动态码确认，管理员可重置；TOTP 密钥 AES-GCM 加密存储
- **API 接口目录**：Token 页内置全部 60+ 接口分组展示，带 METHOD 徽标、中文说明、curl 示例一键复制与搜索过滤
- **配置导出**：域名列表、单 Zone 解析记录一键导出 CSV（UTF-8 BOM，Excel 直开不乱码）
- **审计增强**：操作列统一中文徽标；新增 Zone 精确筛选；`dns.update` 记录变更前后状态 diff，详情弹窗可视化对比
- **移动端适配**：≤768px 侧边栏收进抽屉，弹窗与工具栏自适应

### 修复

- **阿里云 CDN 部署**：补必填参数 `SSLProtocol=on`（此前报 MissingSSLProtocol）；私钥为 PKCS#1 格式时自动转 PKCS#8
- **腾讯云 CDN 部署**：接口名改为 `UploadCertificate`（旧名 `UploadServerCertificate` 在 SSL 2019-12-05 版已下线，报 InvalidAction），响应字段同步更新
- **配置导出文件名**：`Content-Disposition` 引号未闭合导致文件名后缀异常（`csv_`），改为 ASCII 兜底名 + `filename*=UTF-8''` 中文名
- **2FA 登录不跳转**：动态码验证通过后正式 token 未落盘，导致路由守卫拦回登录页

### 运维健壮性

- **`/healthz` 健康检查**：免鉴权探活接口，带 DB 连通性探测（2s 超时），DB 异常返回 503
- **启动 fail-fast**：`config.yaml` 缺失且未显式设置 `DOMHUB_DB_DRIVER` / `DOMHUB_DB_DSN` 时拒绝启动，杜绝从错误工作目录启动时静默回退 sqlite 空库
- **CI**：Go 版本与 go.mod 对齐（1.25 → 1.26）

## [v0.7.0] - 2026-09-14

### 新增

- **证书申请（ACME）**：免费证书自动签发，DNS-01 验证复用已接入云账号的解析通道，支持泛域名；Let's Encrypt 生产/测试环境、ZeroSSL（EAB）多 CA；到期前 30 天自动续期（08:30 每日检查）
- **证书部署**：签发/续期成功后自动下发——阿里云 CDN / 腾讯云 CDN 证书上传与域名绑定（复用云账号 AK），SSH 主机写入证书/私钥并执行 reload 命令；支持多部署目标、手动执行、失败通知告警渠道
- **SSL 证书监控**：从域名与解析记录自动发现监控主机，TLS 拨测读取到期时间，多档位到期告警，支持手动添加与排除
- **域名归属视图**：域名列表展示各域名由哪些账号托管解析（同步中/未接管状态区分），启动时缓存空窗自动预热
- **批量操作与记录模板**：解析记录多选批量改 TTL / 批量删除（NS 保护）；记录模板（A/CNAME/MX/SPF 组合）跨 Zone 一键下发，带预览确认
- **Zone 批量授权**：用户授权页支持筛选、全选、批量勾选 Zone
- **跨 Zone 全局搜索**：顶栏搜索框，授权感知地搜 Zone 与解析记录
- **仪表盘增强**：厂商分布、30 天到期时间线、最近变更时间轴
- **Cloudflare 接入**：域名台账、DNS 解析管理、橙云代理（Proxied）全链路支持
- **深浅色模式**：顶栏一键切换，跟随 localStorage 记忆，防闪烁

### 优化

- 前端视觉重设计：现代浅色风格、靛蓝主色、卡片描边、登录页重做
- 导航重构：域名管理/证书管理前置，新增「安全配置」分组（审计日志、用户与权限）
- 解析记录本地镜像：Zone 列表与记录列表走缓存秒开，变更后自动回源刷新
- 解析快照：查看记录时自动留档（每 Zone 保留 50 份），任意两份快照比较、一键生成恢复计划并执行（含 NS 高危强确认）
- 定时漂移检测：现网与快照有差异时自动留档新基线并告警，避免重复通知

### 修复

- 修复定时任务 cert_check 传 nil context 导致的 panic（08:00 首次真实触发时崩溃）
- 修复通配符证书 DNS-01 验证记录写到 `_acme-challenge.*.example.com` 的 FQDN 错误
- 修复 Zone 缓存空窗期域名归属列误报「未接管」
- 修复侧边栏一级导航未左对齐（CSS 后代选择器误伤）

## [v0.5.0] - 2026-09 初

- Zone 元数据本地缓存，列表秒开
- API Token（`dht_` 前缀），CI / 自动化可编程调用
- 告警渠道测试发送（钉钉 / 企业微信 / 邮件 / Webhook / Telegram）
- 体验补丁包与 README 重写

## [v0.4.0 及之前]

- M0：项目骨架、JWT 登录、CI / 部署配置
- M1：多云账号接入（腾讯云 / 阿里云 / AWS）、域名台账、到期告警
- M2：DNS 解析管理（preview → push）、审计日志
- M3：RBAC 三角色、Zone 级授权
- M4：解析快照、漂移检测、域名 Tags、系统设置、GitHub OAuth
