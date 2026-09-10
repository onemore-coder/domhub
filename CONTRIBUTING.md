# 贡献指南

感谢关注 DomHub！欢迎通过 Issue 反馈问题、提交 PR 参与开发。

## 开发环境

- Go ≥ 1.25，Node ≥ 22
- 数据库：开发默认 SQLite（零配置）；也可用 MySQL，见 `config.example.yaml`

```bash
# 后端
go run ./cmd/server          # http://localhost:8080，默认 admin/admin123

# 前端（热更新）
cd web && npm install && npm run dev   # http://localhost:5173

# 构建验证
go build ./... && go vet ./...
cd web && npm run build
```

## 分支与提交

- 从 `main` 切出功能分支：`feat/xxx`、`fix/xxx`
- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)：
  - `feat(dns): ...`、`fix(alert): ...`、`docs: ...`、`refactor: ...`
- 一个 PR 聚焦一件事，保持可审阅的粒度

## 代码约定

- **后端**：分层为 handler → service → repo，业务逻辑不进 handler；对外部云 API 的访问统一走 `internal/provider` 抽象（`DomainProvider` / `DNSProvider`），新增厂商实现 `Register` / `RegisterDNS` 自注册即可
- **凭证安全**：任何密钥不得明文入库、不得写入日志；SecretKey 永不回传前端
- **DNS 变更类操作**必须经过 plan/push 流程并记录审计日志
- **前端**：页面在 `web/src/views`，API 封装统一放 `web/src/api/domhub.js`
- **定时任务**：新增任务需在 `internal/job/cron.go` 注册，并在 `internal/service/settings.go` 提供默认 cron 与设置页配置项

## 测试

- 提交前确保 `go vet ./...` 与 `go test ./...` 通过
- 涉及云厂商 API 的改动请在真实账号上验证后再提 PR（注意不要在生产域名上测试删除操作）
- CI（GitHub Actions）会在 PR 上跑前后端构建与测试

## 报告 Bug

Issue 请包含：

1. 使用的版本（或 commit）
2. 部署方式（二进制 / Docker）与数据库类型
3. 复现步骤与期望行为
4. 相关日志（**注意脱敏：日志里不要出现 AK/SK、Token**）

## 安全问题

请勿通过公开 Issue 报告安全漏洞，联系仓库维护者私下处理。
