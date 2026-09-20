---
name: domhub
description: DomHub 多云域名/DNS/证书管理台的 MCP 工具使用指南。当用户要求查询域名、管理 DNS 解析记录、检查证书到期、或通过 DomHub MCP 执行任何域名运维操作时使用本 skill。触发词：域名、DNS、解析记录、证书、DomHub、 Zone、CNAME、A 记录。
---

# DomHub MCP 使用指南

用户已通过 MCP 接入 DomHub（多云域名/DNS/证书管理台）。本 skill 指导你正确、安全地使用 DomHub 的 MCP 工具。

## 工具地图

只读工具（readonly / readwrite Token 均可用）：

| 工具 | 用途 |
|---|---|
| `overview` | 全局概览：厂商分布、30 天内到期域名/证书数——一切问题的起点 |
| `list_domains` | 域名台账（注册商维度，含注册到期时间） |
| `list_zones` | 托管解析的 Zone 列表（DNS 解析维度） |
| `list_dns_records` | 查询解析记录（支持 account_id / zone / record_type / keyword 过滤） |
| `search_dns` | 不知道记录在哪个 Zone 时，先用它全局搜 |
| `list_certs` | SSL 证书监控状态（含剩余天数，`expiring_days` 过滤临期） |
| `list_cloud_accounts` | 云账号元信息（密钥已脱敏，不要向用户索要密钥） |
| `list_alert_logs` | 告警发送历史 |

写工具（**仅 readwrite Token 可用**，readonly 调用会报错）：

| 工具 | 用途 |
|---|---|
| `create_dns_record` | 创建解析记录（zone + name + type + value 必填） |
| `update_dns_record` | 按 `record_id` 改记录值 / TTL / 优先级 / 代理状态 |
| `delete_dns_record` | 按 `record_id` 删除记录 |

## 标准工作流

1. **查询类请求**：不确定范围时先调 `overview`；问具体解析用 `list_dns_records`（已知 Zone）或 `search_dns`（不知道在哪）。
2. **修改/删除前必须先查**：写操作用 `record_id` 定位（是 `list_dns_records` 返回的镜像 ID，不是厂商记录 ID）。先查出目标记录、把现状念给用户听，再执行。
3. **改完要验证**：写操作成功后，等几秒再 `list_dns_records` 复查该 Zone，确认镜像已刷新、内容符合预期，把前后对比展示给用户。

## 安全红线

- **删除操作必须先获用户明确确认**，展示将被删除记录的完整信息（zone / name / type / value）。
- **NS / SOA 记录默认不碰**——删除或修改它们可能导致域名解析瘫痪；用户明确要求时，先警告后果再执行。
- **批量变更先小后大**：多 Zone 或多条记录操作时，先改一条让用户验证，确认无误后再继续。
- **不给用户造成"AI 可以随便动生产 DNS"的印象**：涉及生产环境记录（如 @、www、MX）时格外谨慎，主动二次确认。

## 参数约定

- TTL：默认 600 秒；CDN/频繁变更场景可用 60~300；用户未指定时用 600 并告知。
- 主机记录：根域用 `@`，子域只写前缀（`www` 而非 `www.example.com`）。
- MX 记录记得带 `priority`（如 10）。
- 只知道 Zone 名不知道账号时，不用问用户——工具会按 Zone 自动解析归属账号。
- 遇到 "readonly" 权限报错时，告知用户需在 DomHub「安全设置 → API Token」生成读写（readwrite）令牌并更新 MCP 配置。

## 回答风格

- 报数字时带上下文（"30 天内到期 3 个域名，其中 example.com 还剩 5 天"），不要只丢 JSON。
- 展示解析记录用表格：主机记录 / 类型 / 值 / TTL。
- 操作类回答末尾说明"已写入审计日志，可在 DomHub 审计页追溯"。
