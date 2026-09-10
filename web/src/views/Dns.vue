<template>
  <div class="dns-page">
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <el-select v-model="accountFilter" style="width: 190px">
          <el-option :value="0" label="全部账号" />
          <el-option v-for="a in accounts" :key="a.id" :value="a.id" :label="`${a.name}（${a.provider}）`" />
        </el-select>
        <el-input
          v-model="keyword" placeholder="搜索域名" style="width: 260px" clearable
          :prefix-icon="Search"
        />
        <el-button :icon="Refresh" :loading="refreshing" @click="doRefresh()">同步缓存</el-button>
        <div class="spacer" />
        <span v-if="!loading && zonesAll.length" class="zones-summary">
          共 {{ filteredZones.length }} 个托管域名，来自 {{ zoneAccountCount }} 个云账号
          <template v-if="latestSyncedAt">· 缓存更新于 {{ relativeTime(latestSyncedAt) }}</template>
        </span>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table :data="filteredZones" v-loading="loading" stripe @row-click="goRecords">
        <el-table-column label="托管域名" min-width="220">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click.stop="goRecords(row)">{{ row.name }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="云账号" min-width="160">
          <template #default="{ row }">{{ row.account_name }}</template>
        </el-table-column>
        <el-table-column label="厂商" width="110">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ providerLabel(row.provider) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="解析记录" prop="record_count" width="100" align="center" />
        <el-table-column label="缓存时间" width="170">
          <template #default="{ row }">
            <span class="synced-at">{{ formatTime(row.synced_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="goRecords(row)">解析</el-button>
            <el-button link type="warning" @click.stop="goRecords(row, true)">快照</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty
            :description="accounts.length
              ? (keyword || accountFilter ? '没有匹配的托管域名' : '暂无缓存数据，点击「同步缓存」从云厂商拉取')
              : '请先在「云账号」页接入云账号'"
          />
        </template>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listAccounts, listDNSZones, refreshDNSZones } from '../api/domhub'

const route = useRoute()
const router = useRouter()

const accounts = ref([])
const accountFilter = ref(0) // 0 = 全部账号
const keyword = ref('')
const zonesAll = ref([]) // 缓存视图：{ id, cloud_account_id, account_name, provider, name, record_count, synced_at }
const loading = ref(false)
const refreshing = ref(false)

const providerLabel = (p) => ({ aliyun: '阿里云', tencent: '腾讯云', aws: 'AWS' }[p] || p)

const filteredZones = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return zonesAll.value
    .filter((z) => !accountFilter.value || z.cloud_account_id === accountFilter.value)
    .filter((z) => !kw || z.name.toLowerCase().includes(kw))
})

const zoneAccountCount = computed(() => new Set(filteredZones.value.map((z) => z.cloud_account_id)).size)
const latestSyncedAt = computed(() =>
  filteredZones.value.reduce((acc, z) => (z.synced_at > acc ? z.synced_at : acc), ''))

onMounted(async () => {
  try {
    const res = await listAccounts()
    accounts.value = (res.data?.items || []).filter((a) => a.status === 1)
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取云账号列表失败')
  }
  await loadCachedZones()
  // 从详情页改完记录跳回时，静默刷新该账号的记录数
  if (route.query.refresh_account) {
    doRefresh(Number(route.query.refresh_account), true)
  }
})

// 读取本地缓存（不发厂商 API 请求，秒开）
async function loadCachedZones() {
  loading.value = true
  try {
    const res = await listDNSZones()
    zonesAll.value = res.data || []
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取托管域名缓存失败')
  } finally {
    loading.value = false
  }
}

// 同步缓存：回源各厂商 API（后端串行限流），完成后重载列表。
// accountID=0 刷新全部；silent 用于返回页面时的静默单账号刷新。
async function doRefresh(accountID = 0, silent = false) {
  refreshing.value = true
  try {
    const res = await refreshDNSZones(accountID)
    const { accounts: accs, zones } = res.data || {}
    if (!silent) {
      ElMessage.success(`已同步 ${accs || 0} 个账号、${zones || 0} 个托管域名`)
    }
    await loadCachedZones()
  } catch (e) {
    if (!silent) {
      ElMessage.error(e.response?.data?.message || '同步缓存失败')
    }
    await loadCachedZones()
  } finally {
    refreshing.value = false
  }
}

function formatTime(t) {
  if (!t) return '—'
  return t.replace('T', ' ').slice(0, 19)
}

function relativeTime(t) {
  if (!t) return ''
  const diff = (Date.now() - new Date(t).getTime()) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  return `${Math.floor(diff / 86400)} 天前`
}

function goRecords(row, withSnapshot) {
  const query = { account_id: row.cloud_account_id, zone: row.name }
  if (withSnapshot) query.snapshot = '1'
  router.push({ path: '/dns/records', query })
}
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
.spacer {
  flex: 1;
}
.zones-summary {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.synced-at {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
:deep(.el-table__row) {
  cursor: pointer;
}
</style>
