<template>
  <div class="audit-page">
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <el-select v-model="action" placeholder="操作类型" clearable style="width: 180px" @change="load(1)">
          <el-option label="DNS 全部" value="dns" />
          <el-option label="DNS 新增" value="dns.create" />
          <el-option label="DNS 修改" value="dns.update" />
          <el-option label="DNS 删除" value="dns.delete" />
          <el-option label="DNS 批量" value="dns.push" />
          <el-option label="证书部署" value="cert.deploy" />
          <el-option label="用户管理" value="user" />
          <el-option label="Zone 授权" value="user.grant" />
        </el-select>
        <el-input
          v-model="zone" placeholder="按域名筛选" clearable style="width: 180px"
          @keyup.enter="load(1)" @clear="load(1)"
        />
        <el-input
          v-model="keyword" placeholder="搜索资源 / 详情" clearable style="width: 200px"
          @keyup.enter="load(1)" @clear="load(1)"
        />
        <el-button :icon="Search" @click="load(1)">查询</el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table :data="items" v-loading="loading" stripe>
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作人" prop="username" width="110" />
        <el-table-column label="操作" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="actionTag(row.action)">{{ actionLabel(row.action) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="资源" prop="resource" min-width="220" show-overflow-tooltip />
        <el-table-column label="详情" min-width="220">
          <template #default="{ row }">
            <span
              class="detail detail-link" :title="row.detail"
              @click="openDetail(row)"
            >{{ briefDetail(row.detail) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="结果" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'">
              {{ row.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="失败原因" prop="message" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.message || '—' }}</template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无审计日志" />
        </template>
      </el-table>
      <div class="pager">
        <el-pagination
          v-model:current-page="page" :page-size="pageSize" :total="total"
          layout="total, prev, pager, next" @current-change="load()"
        />
      </div>
    </el-card>

    <!-- 详情 / 变更 diff -->
    <el-dialog v-model="detailVisible" title="操作详情" width="680px">
      <div v-if="detailRow">
        <div class="detail-head">
          <el-tag size="small" :type="actionTag(detailRow.action)">{{ actionLabel(detailRow.action) }}</el-tag>
          <span class="detail-res">{{ detailRow.resource }}</span>
          <el-tag size="small" :type="detailRow.status === 'success' ? 'success' : 'danger'">
            {{ detailRow.status === 'success' ? '成功' : '失败' }}
          </el-tag>
        </div>
        <div v-if="detailRow.message" class="detail-err">{{ detailRow.message }}</div>

        <!-- 变更前后 diff（dns.update） -->
        <template v-if="diffPairs">
          <div class="diff-title">变更对比</div>
          <table class="diff-table">
            <thead>
              <tr><th>字段</th><th>变更前</th><th>变更后</th></tr>
            </thead>
            <tbody>
              <tr v-for="p in diffPairs" :key="p.label" :class="{ changed: p.before !== p.after }">
                <td class="diff-field">{{ p.label }}</td>
                <td class="diff-val old">{{ p.before || '—' }}</td>
                <td class="diff-val new">{{ p.after || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </template>

        <!-- 其他动作展示格式化 JSON -->
        <pre v-else class="detail-json">{{ prettyDetail }}</pre>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { listAuditLogs } from '../api/domhub'

const items = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const action = ref('')
const zone = ref('')
const keyword = ref('')

const ACTION_LABELS = {
  'dns.create': 'DNS 新增',
  'dns.update': 'DNS 修改',
  'dns.delete': 'DNS 删除',
  'dns.push': 'DNS 批量',
  'cert.deploy': '证书部署',
  'user.create': '新增用户',
  'user.update': '修改用户',
  'user.delete': '删除用户',
  'user.grant': 'Zone 授权',
}

const actionLabel = (a) => ACTION_LABELS[a] || a

const actionTag = (a) =>
  ({
    'dns.create': 'success',
    'dns.update': 'warning',
    'dns.delete': 'danger',
    'dns.push': 'warning',
    'cert.deploy': 'primary',
    'user.create': 'success',
    'user.update': 'warning',
    'user.delete': 'danger',
    'user.grant': 'primary',
  }[a] || 'info')

function formatTime(t) {
  if (!t) return '—'
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

function briefDetail(d) {
  if (!d) return '—'
  try {
    const obj = JSON.parse(d)
    if (obj.before && obj.after) return '点击查看变更对比'
    const parts = [obj.type, obj.name, obj.value].filter(Boolean)
    return parts.join(' ').slice(0, 80) || d.slice(0, 80)
  } catch {
    return d.slice(0, 80)
  }
}

// ---- 详情弹窗 / diff ----
const detailVisible = ref(false)
const detailRow = ref(null)

function openDetail(row) {
  detailRow.value = row
  detailVisible.value = true
}

// 解析 before/after 为字段对比行（仅 dns.update 且详情含前后状态时）
const diffPairs = computed(() => {
  if (!detailRow.value || detailRow.value.action !== 'dns.update') return null
  try {
    const obj = JSON.parse(detailRow.value.detail || '{}')
    if (!obj.before || !obj.after) return null
    const fields = [
      ['主机记录', 'name'], ['类型', 'type'], ['记录值', 'value'],
      ['TTL', 'ttl'], ['优先级', 'priority'], ['线路', 'line'],
      ['状态', 'status'], ['备注', 'remark'], ['云代理', 'proxied'],
    ]
    const rows = []
    for (const [label, key] of fields) {
      const b = String(obj.before[key] ?? '')
      const a = String(obj.after[key] ?? '')
      if (b !== '' || a !== '') rows.push({ label, before: b, after: a })
    }
    return rows.length ? rows : null
  } catch {
    return null
  }
})

const prettyDetail = computed(() => {
  if (!detailRow.value?.detail) return '—'
  try {
    return JSON.stringify(JSON.parse(detailRow.value.detail), null, 2)
  } catch {
    return detailRow.value.detail
  }
})

async function load(p) {
  if (p) page.value = p
  loading.value = true
  try {
    const res = await listAuditLogs({
      page: page.value, page_size: pageSize,
      action: action.value, zone: zone.value, keyword: keyword.value,
    })
    items.value = res.data?.items || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(() => load(1))
</script>

<style scoped>
.toolbar-card {
  margin-bottom: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.detail {
  color: var(--el-text-color-regular);
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
}
.detail-link {
  cursor: pointer;
}
.detail-link:hover {
  color: var(--el-color-primary);
}
.detail-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.detail-res {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
  word-break: break-all;
}
.detail-err {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--el-color-danger-light-9);
  color: var(--el-color-danger);
  font-size: 13px;
}
.diff-title {
  margin: 6px 0 8px;
  font-weight: 600;
  font-size: 13px;
}
.diff-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.diff-table th,
.diff-table td {
  padding: 7px 10px;
  border: 1px solid var(--el-border-color-lighter);
  text-align: left;
  word-break: break-all;
}
.diff-table th {
  background: var(--el-fill-color-light);
  font-weight: 600;
}
.diff-table tr.changed .diff-field {
  color: var(--el-color-warning);
  font-weight: 600;
}
.diff-val {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
}
.diff-table tr.changed .diff-val.old {
  color: var(--el-color-danger);
  text-decoration: line-through;
}
.diff-table tr.changed .diff-val.new {
  color: var(--el-color-success);
  font-weight: 600;
}
.detail-json {
  margin: 0;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
  max-height: 380px;
  overflow: auto;
}
</style>
