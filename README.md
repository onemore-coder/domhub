# DomHub

> 多云域名与 DNS 统一管理平台（开发中 · M0 骨架阶段）

把散落在腾讯云、阿里云、AWS 等多个云厂商、多个账号下的域名和 DNS 解析，收拢到一个控制台里统一管理。

## 技术栈

- **后端**：Go + Gin + GORM + MySQL（开发模式支持 SQLite）
- **前端**：Vue 3 + Element Plus + Vite + Pinia
- **部署**：单二进制（内嵌前端）/ Docker Compose

## 功能规划（Roadmap）

| 里程碑 | 内容 | 状态 |
|---|---|---|
| M0 | 项目骨架、登录认证（JWT）、CI、部署配置 | ✅ 进行中 |
| M1 | 云账号接入、域名台账同步、到期告警 | 🚧 |
| M2 | DNS 解析管理（preview → push）、审计日志 | |
| M3 | RBAC、批量操作、通知渠道 | |
| M4 | 文档完善、v1.0 正式发布 | |

## 本地开发

### 后端

```bash
# 默认使用 SQLite（domhub.db），无需 MySQL
go run ./cmd/server
# 服务监听 http://localhost:8080
# 默认账号 admin / admin123（可由 config.yaml 或环境变量覆盖）
```

生产配置：`cp config.example.yaml config.yaml` 后修改，或使用 `DOMHUB_*` 环境变量。

### 前端

```bash
cd web
npm install
npm run dev        # 开发模式 http://localhost:5173，API 代理到 8080
npm run build      # 构建产物输出到 web/dist（后端构建时会内嵌）
```

### Docker Compose 一键部署

```bash
docker compose up -d
# 访问 http://localhost:8080
```

## API 概要

```
POST /api/v1/auth/login          登录（返回 JWT）
GET  /api/v1/auth/me             当前用户信息
POST /api/v1/auth/logout         登出
GET  /api/v1/dashboard/summary   仪表盘统计
```

## 项目结构

```
domhub/
├── cmd/server/          # 入口
├── internal/
│   ├── api/             # 路由 / handler / middleware / dto
│   ├── bootstrap/       # 数据库初始化与种子数据
│   ├── model/           # GORM 模型
│   ├── repo/            # 数据访问层
│   ├── service/         # 业务逻辑层
│   └── pkg/             # config / logger / jwtx
├── web/                 # Vue3 + Element Plus 前端
├── deploy/              # 部署脚本（预留）
└── .github/workflows/   # CI（构建 / 测试 / 发版）
```

## License

Apache-2.0
