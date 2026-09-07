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
        <el-button type="primary" plain :loading="syncing" @click="doSyncAll">
          <el-icon><Refresh /></el-icon>&nbsp;同步全部账号
        </el-button>
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
        <el-table-column label="标签" min-width="120">
          <template #default="{ row }">
            <el-tag v-for="t in splitTags(row.tags)" :key="t" size="small" class="tag-item">{{ t }}</el-tag>
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
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listDomains, syncAllDomains } from '../api/domhub'

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
  color: #1f2d3d;
}
.expire-danger {
  color: #f56c6c;
  font-weight: 600;
}
.expire-warn {
  color: #e6a23c;
  font-weight: 600;
}
.days-tag {
  margin-left: 6px;
}
.tag-item {
  margin-right: 4px;
}
.text-muted {
  color: #c0c4cc;
}
.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
