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
        <el-button :icon="Refresh" :loading="loading" @click="loadAllZones">刷新</el-button>
        <div class="spacer" />
        <span v-if="!loading && zonesAll.length" class="zones-summary">
          共 {{ filteredZones.length }} 个托管域名，来自 {{ zoneAccountCount }} 个云账号
        </span>
        <span v-else-if="loading" class="zones-summary">正在拉取各账号托管域名…（{{ progress }}/{{ accounts.length }}）</span>
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
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="goRecords(row)">解析</el-button>
            <el-button link type="warning" @click.stop="goRecords(row, true)">快照</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty
            :description="accounts.length
              ? (keyword || accountFilter ? '没有匹配的托管域名' : '所有账号下均没有托管解析的域名（域名可能未开启云解析）')
              : '请先在「云账号」页接入云账号'"
          />
        </template>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listAccounts, listDNSZones } from '../api/domhub'

const router = useRouter()

const accounts = ref([])
const accountFilter = ref(0) // 0 = 全部账号
const keyword = ref('')
const zonesAll = ref([]) // 聚合后的扁平 Zone 列表：{ account_id, account_name, provider, name, record_count }
const loading = ref(false)
const progress = ref(0)

const providerLabel = (p) => ({ aliyun: '阿里云', tencent: '腾讯云', aws: 'AWS' }[p] || p)

const filteredZones = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return zonesAll.value
    .filter((z) => !accountFilter.value || z.account_id === accountFilter.value)
    .filter((z) => !kw || z.name.toLowerCase().includes(kw))
})

const zoneAccountCount = computed(() => new Set(filteredZones.value.map((z) => z.account_id)).size)

onMounted(async () => {
  try {
    const res = await listAccounts()
    accounts.value = (res.data?.items || []).filter((a) => a.status === 1)
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取云账号列表失败')
    return
  }
  await loadAllZones()
})

// 并行拉取全部有权限账号的托管域名，聚合成扁平列表。
// 单个账号失败不阻塞整体，最后统一警告。
async function loadAllZones() {
  if (!accounts.value.length) return
  loading.value = true
  progress.value = 0
  const failed = []
  const results = await Promise.all(
    accounts.value.map((a) =>
      listDNSZones(a.id)
        .then((res) => ({ account: a, zones: res.data || [] }))
        .catch(() => {
          failed.push(a.name)
          return { account: a, zones: [] }
        })
        .finally(() => { progress.value++ }),
    ),
  )
  zonesAll.value = results.flatMap(({ account, zones }) =>
    zones.map((z) => ({
      account_id: account.id,
      account_name: account.name,
      provider: account.provider,
      name: z.name,
      record_count: z.record_count,
    })))
  loading.value = false
  if (failed.length) {
    ElMessage.warning(`以下账号的托管域名拉取失败，已跳过：${failed.join('、')}（可点刷新重试）`)
  }
}

function goRecords(row, withSnapshot) {
  const query = { account_id: row.account_id, zone: row.name }
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
:deep(.el-table__row) {
  cursor: pointer;
}
</style>
