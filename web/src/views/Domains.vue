<template>
  <div>
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <div class="filters">
          <el-input v-model="filters.keyword" placeholder="搜索域名 / 备注" clearable style="width: 220px" @keyup.enter="load" @clear="load" />
          <el-select v-model="filters.provider" placeholder="厂商" clearable style="width: 130px" @change="load">
            <el-option label="腾讯云" value="tencent" />
            <el-option label="阿里云" value="aliyun" />
            <el-option label="AWS" value="aws" />
          </el-select>
          <el-select v-model="filters.kind" placeholder="类型" clearable style="width: 120px" @change="load">
            <el-option label="注册域名" value="domain" />
            <el-option label="托管 Zone" value="zone" />
          </el-select>
          <el-select v-model="filters.expiring_days" placeholder="到期时间" clearable style="width: 150px" @change="load">
            <el-option label="30 天内到期" :value="30" />
            <el-option label="60 天内到期" :value="60" />
            <el-option label="90 天内到期" :value="90" />
          </el-select>
          <el-button @click="load">查询</el-button>
        </div>
        <div class="actions">
          <el-button @click="exportCsv">
            <el-icon><Download /></el-icon>&nbsp;导出 CSV
          </el-button>
          <el-button type="primary" plain :loading="syncing" @click="doSyncAll">
            <el-icon><Refresh /></el-icon>&nbsp;同步全部账号
          </el-button>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="domains" stripe>
        <el-table-column prop="name" label="域名" min-width="220">
          <template #default="{ row }">
            <span class="domain-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.kind === 'domain' ? 'primary' : 'info'">
              {{ row.kind === 'domain' ? '注册域名' : '托管Zone' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="厂商" width="100">
          <template #default="{ row }">
            <el-tag size="small" type="warning">{{ row.provider }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="registrar" label="注册商" width="130" />
        <el-table-column label="到期时间" width="170">
          <template #default="{ row }">
            <template v-if="row.expire_at">
              <span :class="expireClass(row.expire_at)">{{ formatDate(row.expire_at) }}</span>
              <el-tag v-if="daysLeft(row.expire_at) <= 30" size="small" type="danger" class="days-tag">
                剩 {{ daysLeft(row.expire_at) }} 天
              </el-tag>
            </template>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="解析归属" min-width="180">
          <template #default="{ row }">
            <template v-if="zoneOwners[row.name]?.length">
              <el-tooltip placement="top">
                <template #content>
                  <div v-for="o in zoneOwners[row.name]" :key="o.cloud_account_id">
                    {{ o.account_name }}（{{ providerLabel(o.provider) }}）· {{ o.record_count }} 条记录
                  </div>
                </template>
                <span class="owner-tags">
                  <el-tag
                    v-for="o in zoneOwners[row.name]" :key="o.cloud_account_id"
                    size="small" type="success" class="owner-tag clickable"
                    @click="goZone(o, row)"
                  >{{ providerLabel(o.provider) }}</el-tag>
                </span>
              </el-tooltip>
            </template>
            <el-tooltip v-else-if="zonesSynced && row.kind === 'domain'" placement="top"
              content="该域名的解析未托管在任何已接入账号，如需管理请把解析迁移到已接入厂商或接入对应账号">
              <el-tag size="small" type="danger" class="owner-tag">未接管</el-tag>
            </el-tooltip>
            <el-tooltip v-else-if="row.kind === 'domain'" placement="top"
              content="Zone 缓存尚未同步完成，暂无法判断解析归属；稍后自动刷新，或到 DNS 管理页手动刷新">
              <el-tag size="small" type="info" class="owner-tag">同步中</el-tag>
            </el-tooltip>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="标签" min-width="140">
          <template #default="{ row }">
            <el-tag v-for="t in splitTags(row.tags)" :key="t" size="small" class="tag-item">{{ t }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openMeta(row)">编辑</el-button>
          </template>
        </el-table-column>
        <el-table-column label="最近同步" width="170">
          <template #default="{ row }">{{ formatTime(row.last_synced_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="filters.page"
        :page-size="filters.page_size"
        :total="total"
        layout="total, prev, pager, next"
        class="pagination"
        @current-change="load"
      />
    </el-card>

    <!-- 标签/备注编辑 -->
    <el-dialog v-model="metaDialog" title="编辑标签与备注" width="440px">
      <el-form label-width="60px">
        <el-form-item label="标签">
          <el-input v-model="metaForm.tags" placeholder="逗号分隔，如：生产,核心" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="metaForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="metaDialog = false">取消</el-button>
        <el-button type="primary" :loading="metaSaving" @click="saveMeta">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Download, Refresh } from '@element-plus/icons-vue'
import { listDomains, syncAllDomains, updateDomainMeta } from '../api/domhub'

const router = useRouter()
const zoneOwners = ref({}) // 域名 → 托管其解析的账号列表
const zonesSynced = ref(true) // Zone 缓存是否已同步（为空时「未接管」不可信）
const providerLabel = (p) => ({ aliyun: '阿里云', tencent: '腾讯云', aws: 'AWS', cloudflare: 'Cloudflare' }[p] || p)

// 点击归属标签跳转到对应 Zone 的解析记录页
function goZone(owner, row) {
  router.push({ path: '/dns/records', query: { account_id: owner.cloud_account_id, zone: row.name } })
}

const loading = ref(false)
const syncing = ref(false)
const domains = ref([])
const total = ref(0)

const filters = reactive({
  keyword: '',
  provider: '',
  kind: '',
  expiring_days: null,
  page: 1,
  page_size: 20,
})

// 导出 CSV（带当前筛选条件）：走 fetch + Bearer 头，浏览器原生跳转带不上 Authorization
async function exportCsv() {
  const qs = new URLSearchParams()
  if (filters.keyword) qs.set('keyword', filters.keyword)
  if (filters.provider) qs.set('provider', filters.provider)
  if (filters.kind) qs.set('kind', filters.kind)
  if (filters.expiring_days) qs.set('expiring_days', filters.expiring_days)
  try {
    const res = await fetch(`/api/v1/domains/export?${qs.toString()}`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('domhub_token')}`, 'X-Api-Key': localStorage.getItem('domhub_token') },
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = decodeURIComponent((res.headers.get('Content-Disposition') || '').match(/filename\*=UTF-8''(.+)/)?.[1] || 'domains.csv')
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.error('导出失败，请稍后重试')
  }
}

const formatDate = (t) => (t ? new Date(t).toLocaleDateString('zh-CN') : '—')
const formatTime = (t) => (t ? new Date(t).toLocaleString('zh-CN') : '—')
const splitTags = (tags) => (tags ? tags.split(',').filter(Boolean) : [])

function daysLeft(t) {
  return Math.ceil((new Date(t).getTime() - Date.now()) / 86400000)
}

function expireClass(t) {
  const d = daysLeft(t)
  if (d <= 30) return 'expire-danger'
  if (d <= 90) return 'expire-warn'
  return ''
}

async function load() {
  loading.value = true
  try {
    const res = await listDomains({ ...filters, expiring_days: filters.expiring_days || undefined })
    domains.value = res.data.items
    total.value = res.data.total
    zoneOwners.value = res.data.zone_owners || {}
    zonesSynced.value = Object.keys(zoneOwners.value).length > 0
  } finally {
    loading.value = false
  }
}

async function doSyncAll() {
  syncing.value = true
  try {
    await syncAllDomains()
    ElMessage.success('同步任务已触发，稍后刷新查看')
  } finally {
    syncing.value = false
  }
}

// ---- 标签/备注 ----
const metaDialog = ref(false)
const metaSaving = ref(false)
const metaForm = ref({ id: 0, tags: '', remark: '' })

function openMeta(row) {
  metaForm.value = { id: row.id, tags: row.tags || '', remark: row.remark || '' }
  metaDialog.value = true
}

async function saveMeta() {
  metaSaving.value = true
  try {
    await updateDomainMeta(metaForm.value.id, {
      tags: metaForm.value.tags.trim(),
      remark: metaForm.value.remark,
    })
    ElMessage.success('已保存')
    metaDialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    metaSaving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar-card {
  margin-bottom: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.filters {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.domain-name {
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.expire-danger {
  color: #dc2626;
  font-weight: 600;
}
.expire-warn {
  color: #d97706;
  font-weight: 600;
}
.days-tag {
  margin-left: 6px;
}
.tag-item {
  margin-right: 4px;
}
.text-muted {
  color: var(--el-text-color-placeholder);
}
.owner-tags {
  display: inline-flex;
  gap: 4px;
  flex-wrap: wrap;
}
.owner-tag.clickable {
  cursor: pointer;
}
.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
