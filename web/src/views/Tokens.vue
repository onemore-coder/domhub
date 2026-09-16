<template>
  <div>
    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <div>
            <span>API Token</span>
            <span class="hint">用于 CI / 自动化脚本调用 DomHub API，请求头携带 Authorization: Bearer dht_xxx</span>
          </div>
          <el-button size="small" type="primary" @click="openCreate">生成令牌</el-button>
        </div>
      </template>
      <el-table v-loading="loading" :data="tokens" stripe size="small">
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="令牌前缀" width="160">
          <template #default="{ row }">
            <code class="prefix">{{ row.prefix }}…</code>
          </template>
        </el-table-column>
        <el-table-column label="有效期至" width="180">
          <template #default="{ row }">
            <span v-if="row.expire_at">{{ formatTime(row.expire_at) }}</span>
            <el-tag v-else size="small" type="success">永不过期</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近使用" width="180">
          <template #default="{ row }">
            <span v-if="row.last_used_at">{{ formatTime(row.last_used_at) }}</span>
            <span v-else class="muted">从未使用</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" @click="revoke(row)">吊销</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="还没有 API Token，生成一个用于自动化调用" :image-size="70" />
        </template>
      </el-table>
    </el-card>

    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <span>API 接口目录</span>
          <el-input
            v-model="keyword" size="small" clearable placeholder="搜索路径或说明"
            style="width: 220px" :prefix-icon="Search"
          />
        </div>
      </template>

      <div class="api-hint">
        所有接口以 <code>{{ apiBase }}</code> 为前缀，认证头 <code>Authorization: Bearer dht_xxx</code>；
        点击接口行展开 curl 示例，可一键复制（令牌为占位符，替换为你的真实令牌）。
      </div>

      <el-collapse v-model="openModules" class="api-collapse">
        <el-collapse-item v-for="g in filteredGroups" :key="g.name" :name="g.name">
          <template #title>
            <span class="module-name">{{ g.name }}</span>
            <el-tag size="small" type="info" effect="plain" class="module-count">{{ g.items.length }}</el-tag>
          </template>
          <div v-for="ep in g.items" :key="ep.m + ep.p" class="ep">
            <div class="ep-row" @click="toggle(g.name, ep)">
              <el-tag size="small" class="method" :class="'m-' + ep.m.toLowerCase()">{{ ep.m }}</el-tag>
              <code class="ep-path">{{ ep.p }}</code>
              <span class="ep-desc">{{ ep.desc }}</span>
            </div>
            <div v-if="isOpen(g.name, ep)" class="ep-detail">
              <pre class="example">{{ curl(ep) }}</pre>
              <el-button size="small" type="primary" plain @click="copyCurl(ep)">复制 curl</el-button>
            </div>
          </div>
        </el-collapse-item>
        <el-empty v-if="!filteredGroups.length" description="没有匹配的接口" :image-size="60" />
      </el-collapse>
    </el-card>

    <!-- 生成令牌 -->
    <el-dialog v-model="createVisible" title="生成 API Token" width="460">
      <el-form label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：CI 流水线、备份脚本" maxlength="32" />
        </el-form-item>
        <el-form-item label="有效期">
          <el-select v-model="form.expire_days" style="width: 100%">
            <el-option :value="0" label="永不过期" />
            <el-option :value="7" label="7 天" />
            <el-option :value="30" label="30 天" />
            <el-option :value="90" label="90 天" />
            <el-option :value="365" label="365 天" />
          </el-select>
        </el-form-item>
      </el-form>
      <div v-if="created" class="created-box">
        <el-alert type="warning" :closable="false" show-icon
          title="请立即复制保存，此明文仅显示这一次" />
        <code class="plain-token">{{ created }}</code>
        <el-button size="small" type="primary" plain @click="copyToken">复制</el-button>
      </div>
      <template #footer>
        <el-button @click="createVisible = false">{{ created ? '完成' : '取消' }}</el-button>
        <el-button v-if="!created" type="primary" :loading="creating" @click="doCreate">生成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { listTokens, createToken, revokeToken } from '../api/domhub'
import http from '../api/http'

const tokens = ref([])
const loading = ref(false)
const createVisible = ref(false)
const creating = ref(false)
const created = ref('')
const form = ref({ name: '', expire_days: 0 })
const apiBase = `${window.location.origin}/api/v1`

// ---------- API 接口目录（与路由保持同步；body 为 JSON 请求体示例） ----------
const TOKEN = 'dht_你的令牌'
const CATALOG = [
  { name: '仪表盘', items: [
    { m: 'GET', p: '/dashboard/summary', desc: '总览统计（域名/证书数等）' },
    { m: 'GET', p: '/dashboard/stats', desc: '图表数据（厂商分布/到期时间线/最近变更）' },
  ]},
  { name: '云账号', items: [
    { m: 'GET', p: '/accounts', desc: '云账号列表（凭证脱敏）' },
    { m: 'POST', p: '/accounts', desc: '添加云账号', body: { name: '阿里云主号', provider: 'aliyun', access_key: 'AK', secret_key: 'SK' } },
    { m: 'PUT', p: '/accounts/:id', desc: '更新云账号（凭证留空表示沿用）' },
    { m: 'POST', p: '/accounts/:id/check', desc: '验证账号连通性' },
    { m: 'POST', p: '/accounts/:id/sync', desc: '同步该账号的域名/Zone' },
    { m: 'DELETE', p: '/accounts/:id', desc: '删除云账号' },
  ]},
  { name: '域名管理', items: [
    { m: 'GET', p: '/domains', desc: '域名列表（含注册到期/归属 Zone）' },
    { m: 'POST', p: '/domains/sync', desc: '全量同步所有账号域名' },
    { m: 'PATCH', p: '/domains/:id', desc: '更新标签/备注', body: { tags: ['生产'], remark: '主站' } },
  ]},
  { name: 'DNS 解析', items: [
    { m: 'GET', p: '/dns/zones', desc: 'Zone 列表（本地缓存，秒开）' },
    { m: 'POST', p: '/dns/zones/refresh', desc: '强制刷新 Zone 缓存' },
    { m: 'GET', p: '/dns/records', desc: '解析记录（实时读厂商）' },
    { m: 'GET', p: '/dns/records-cached', desc: '解析记录（缓存镜像，快）' },
    { m: 'POST', p: '/dns/records/sync', desc: '同步解析记录到本地缓存' },
    { m: 'POST', p: '/dns/records', desc: '新建解析记录', body: { zone_id: 'xxx', type: 'A', name: 'www', value: '1.2.3.4', ttl: 600 } },
    { m: 'PUT', p: '/dns/records', desc: '修改解析记录', body: { zone_id: 'xxx', record_id: 'yyy', value: '5.6.7.8', ttl: 300 } },
    { m: 'DELETE', p: '/dns/records', desc: '删除解析记录（NS 记录需确认）', body: { zone_id: 'xxx', record_id: 'yyy' } },
    { m: 'POST', p: '/dns/plan', desc: '变更预览（生成 diff 计划，不执行）', body: { zone_id: 'xxx', changes: [] } },
    { m: 'POST', p: '/dns/push', desc: '按计划确认执行变更', body: { plan: {} } },
    { m: 'GET', p: '/search?q=xxx', desc: '全局搜索 Zone 与解析记录（授权感知）' },
  ]},
  { name: '快照与恢复', items: [
    { m: 'GET', p: '/dns/snapshots', desc: '快照列表' },
    { m: 'GET', p: '/dns/snapshots/:id', desc: '快照详情（记录全集）' },
    { m: 'POST', p: '/dns/snapshots', desc: '立即抓取快照', body: { zone_id: 'xxx' } },
    { m: 'POST', p: '/dns/snapshots/diff', desc: '比较两份快照', body: { a_id: 1, b_id: 2 } },
    { m: 'POST', p: '/dns/snapshots/restore-plan', desc: '生成恢复计划（预览，需再走 /dns/push）', body: { snapshot_id: 1 } },
  ]},
  { name: '记录模板', items: [
    { m: 'GET', p: '/record-templates', desc: '模板列表' },
    { m: 'POST', p: '/record-templates', desc: '创建模板', body: { name: '博客标配', records: [{ type: 'A', name: 'www', value: '1.2.3.4' }] } },
    { m: 'PUT', p: '/record-templates/:id', desc: '更新模板' },
    { m: 'POST', p: '/record-templates/:id/apply', desc: '应用模板到 Zone（返回预览，同名跳过）', body: { zone_id: 'xxx', dry_run: true } },
    { m: 'DELETE', p: '/record-templates/:id', desc: '删除模板' },
  ]},
  { name: '证书监控', items: [
    { m: 'GET', p: '/certs', desc: '监控主机列表与到期时间' },
    { m: 'POST', p: '/certs', desc: '手动添加监控主机', body: { host: 'example.com', port: 443 } },
    { m: 'POST', p: '/certs/check', desc: '全量 TLS 拨测' },
    { m: 'POST', p: '/certs/:id/check', desc: '单主机立即拨测' },
    { m: 'PUT', p: '/certs/:id/excluded', desc: '设置排除监控', body: { excluded: true } },
    { m: 'DELETE', p: '/certs/:id', desc: '删除监控主机' },
  ]},
  { name: '证书签发（ACME）', items: [
    { m: 'GET', p: '/certs-issued/cas', desc: '支持的 CA 列表' },
    { m: 'GET', p: '/certs-issued', desc: '已签发证书列表' },
    { m: 'GET', p: '/certs-issued/:id', desc: '证书详情（含进度日志）' },
    { m: 'POST', p: '/certs-issued/apply', desc: '申请证书（DNS-01，支持泛域名）', body: { primary_domain: 'example.com', san: ['example.com', '*.example.com'], ca: 'letsencrypt', account_id: 1, contact_email: 'a@b.com' } },
    { m: 'POST', p: '/certs-issued/:id/renew', desc: '立即续期' },
    { m: 'PUT', p: '/certs-issued/:id/auto-renew', desc: '开关自动续期', body: { enabled: true } },
    { m: 'GET', p: '/certs-issued/:id/download?type=fullchain', desc: '下载证书（type=fullchain/chain/key）' },
    { m: 'DELETE', p: '/certs-issued/:id', desc: '删除签发记录' },
  ]},
  { name: '证书部署', items: [
    { m: 'GET', p: '/certs-issued/:id/deploys', desc: '某证书的部署目标列表' },
    { m: 'POST', p: '/certs-issued/:id/deploys', desc: '新增部署目标（type=aliyun_cdn/tencent_cdn/ssh）', body: { type: 'aliyun_cdn', name: '主站CDN', account_id: 1, config: { domain: 'www.example.com' } } },
    { m: 'PUT', p: '/certs/deploys/:id', desc: '更新部署目标' },
    { m: 'POST', p: '/certs/deploys/:id/run', desc: '手动执行一次部署' },
    { m: 'DELETE', p: '/certs/deploys/:id', desc: '删除部署目标' },
  ]},
  { name: '告警中心', items: [
    { m: 'GET', p: '/channels', desc: '通知渠道列表' },
    { m: 'POST', p: '/channels', desc: '添加渠道（dingtalk/wecom/email/webhook/telegram）', body: { type: 'dingtalk', name: '运维群', config: { webhook: 'https://oapi.dingtalk.com/robot/send?access_token=xx' } } },
    { m: 'POST', p: '/channels/test', desc: '测试渠道配置（未保存前）', body: { type: 'webhook', config: { url: 'https://...' } } },
    { m: 'POST', p: '/channels/:id/test', desc: '向已保存渠道发测试消息' },
    { m: 'PUT', p: '/channels/:id', desc: '更新渠道' },
    { m: 'DELETE', p: '/channels/:id', desc: '删除渠道' },
    { m: 'GET', p: '/alert-rules', desc: '告警规则列表' },
    { m: 'POST', p: '/alert-rules', desc: '创建告警规则', body: { type: 'domain_expiry', days: [30, 7, 1], channel_id: 1 } },
    { m: 'PUT', p: '/alert-rules/:id', desc: '更新规则' },
    { m: 'DELETE', p: '/alert-rules/:id', desc: '删除规则' },
    { m: 'POST', p: '/alerts/check', desc: '立即执行一次告警检查' },
    { m: 'GET', p: '/alerts/logs', desc: '告警发送日志' },
  ]},
  { name: '用户与授权', items: [
    { m: 'GET', p: '/users', desc: '用户列表（管理员）' },
    { m: 'POST', p: '/users', desc: '创建用户', body: { username: 'dev', password: '***', role: 'viewer' } },
    { m: 'PUT', p: '/users/:id', desc: '更新用户（角色/密码）' },
    { m: 'GET', p: '/users/:id/zones', desc: '查看用户 Zone 授权' },
    { m: 'PUT', p: '/users/:id/zones', desc: '设置用户 Zone 授权', body: { grants: [{ account_id: 1, zone_id: 'xxx' }] } },
    { m: 'DELETE', p: '/users/:id', desc: '删除用户' },
    { m: 'POST', p: '/users/me/password', desc: '修改自己的密码', body: { old_password: '***', new_password: '***' } },
  ]},
  { name: 'API Token', items: [
    { m: 'GET', p: '/tokens', desc: '令牌列表（仅前缀）' },
    { m: 'POST', p: '/tokens', desc: '生成令牌（明文仅返回一次）', body: { name: 'CI 流水线', expire_days: 0 } },
    { m: 'DELETE', p: '/tokens/:id', desc: '吊销令牌' },
  ]},
  { name: '审计与系统', items: [
    { m: 'GET', p: '/audit-logs', desc: '审计日志（支持筛选）' },
    { m: 'GET', p: '/settings', desc: '系统设置（定时任务计划等）' },
    { m: 'PUT', p: '/settings/schedules', desc: '更新定时任务计划（热生效）', body: { sync_zones_cron: '0 0 */2 * * *' } },
  ]},
]

const keyword = ref('')
const openModules = ref([])
const expanded = ref(new Set())

const filteredGroups = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return CATALOG
  return CATALOG
    .map((g) => ({ ...g, items: g.items.filter((e) => (e.p + e.desc + e.m).toLowerCase().includes(k)) }))
    .filter((g) => g.items.length)
})

function epKey(g, ep) {
  return g + '|' + ep.m + ' ' + ep.p
}
function isOpen(g, ep) {
  return expanded.value.has(epKey(g, ep))
}
function toggle(g, ep) {
  const s = new Set(expanded.value)
  const k = epKey(g, ep)
  s.has(k) ? s.delete(k) : s.add(k)
  expanded.value = s
}

function curl(ep) {
  const path = ep.p.replace(':id', '1').replace(':key', '1')
  let cmd = `curl -s -X ${ep.m} '${apiBase}${path}' \\\n  -H 'Authorization: Bearer ${TOKEN}'`
  if (ep.body) {
    cmd += ` \\\n  -H 'Content-Type: application/json' \\\n  -d '${JSON.stringify(ep.body)}'`
  }
  return cmd
}

async function copyCurl(ep) {
  try {
    await navigator.clipboard.writeText(curl(ep))
    ElMessage.success('已复制，替换令牌后即可调用')
  } catch {
    ElMessage.warning('复制失败，请手动选择复制')
  }
}

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await listTokens()
    tokens.value = res.data || []
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取令牌列表失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { name: '', expire_days: 0 }
  created.value = ''
  createVisible.value = true
}

async function doCreate() {
  creating.value = true
  try {
    const res = await createToken(form.value)
    created.value = res.data?.plain_token || ''
    await load()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '生成失败')
  } finally {
    creating.value = false
  }
}

async function copyToken() {
  try {
    await navigator.clipboard.writeText(created.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.warning('复制失败，请手动选择复制')
  }
}

async function revoke(row) {
  try {
    await ElMessageBox.confirm(
      `确认吊销令牌「${row.name}」（${row.prefix}…）？使用它的脚本将立即失效。`,
      '吊销令牌',
      { type: 'warning', confirmButtonText: '吊销', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  try {
    await revokeToken(row.id)
    ElMessage.success('已吊销')
    await load()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '吊销失败')
  }
}

function formatTime(t) {
  return (t || '').replace('T', ' ').slice(0, 19)
}
</script>

<style scoped>
.section {
  margin-bottom: 16px;
}
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.hint {
  margin-left: 12px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.prefix,
.plain-token {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
}
.muted {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.example {
  margin: 0;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
  overflow-x: auto;
}
/* ---- API 接口目录 ---- */
.api-hint {
  margin-bottom: 12px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.7;
}
.api-hint code {
  padding: 1px 5px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
}
.api-collapse :deep(.el-collapse-item__header) {
  font-weight: 600;
}
.module-name {
  font-size: 13px;
}
.module-count {
  margin-left: 8px;
}
.ep {
  border-bottom: 1px dashed var(--el-border-color-lighter);
}
.ep:last-child {
  border-bottom: none;
}
.ep-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 4px;
  cursor: pointer;
  border-radius: 4px;
}
.ep-row:hover {
  background: var(--dh-hover, var(--el-fill-color-light));
}
.method {
  width: 62px;
  text-align: center;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-weight: 700;
  color: #fff;
  border: none;
}
.m-get { background: #409eff; }
.m-post { background: #67c23a; }
.m-put { background: #e6a23c; }
.m-patch { background: #b882e7; }
.m-delete { background: #f56c6c; }
.ep-path {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
  white-space: nowrap;
}
.ep-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ep-detail {
  padding: 4px 4px 12px 72px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}
.created-box {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: flex-start;
}
.plain-token {
  display: block;
  width: 100%;
  padding: 10px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  word-break: break-all;
}
</style>
