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
          v-model="keyword" placeholder="搜索资源 / 详情" clearable style="width: 240px"
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
            <span class="detail" :title="row.detail">{{ briefDetail(row.detail) }}</span>
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
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { listAuditLogs } from '../api/domhub'

const items = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const action = ref('')
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
    const parts = [obj.type, obj.name, obj.value].filter(Boolean)
    return parts.join(' ').slice(0, 80) || d.slice(0, 80)
  } catch {
    return d.slice(0, 80)
  }
}

async function load(p) {
  if (p) page.value = p
  loading.value = true
  try {
    const res = await listAuditLogs({
      page: page.value, page_size: pageSize,
      action: action.value, keyword: keyword.value,
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
</style>
