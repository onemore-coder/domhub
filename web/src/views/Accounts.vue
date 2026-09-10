<template>
  <div>
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <div>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>接入云账号
          </el-button>
          <el-tag v-for="p in connectors" :key="p" type="info" class="conn-tag">{{ p }}</el-tag>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="accounts" stripe>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="厂商" width="110">
          <template #default="{ row }">
            <el-tag :type="providerTagType(row.provider)">{{ row.provider }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="access_key" label="AccessKey" min-width="140" />
        <el-table-column prop="region" label="Region" width="110" />
        <el-table-column label="连通状态" min-width="200">
          <template #default="{ row }">
            <template v-if="row.last_check_at">
              <el-tag :type="row.last_check_ok ? 'success' : 'danger'" size="small">
                {{ row.last_check_ok ? '正常' : '异常' }}
              </el-tag>
              <span class="check-msg">{{ row.last_check_msg }}</span>
            </template>
            <span v-else class="text-muted">未检测</span>
          </template>
        </el-table-column>
        <el-table-column label="上次同步" width="170">
          <template #default="{ row }">
            {{ row.last_sync_at ? formatTime(row.last_sync_at) : '—' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="doCheck(row)">连通检测</el-button>
            <el-button size="small" type="primary" plain @click="doSync(row)">同步</el-button>
            <el-button size="small" @click="doDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="接入云账号" width="520">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：公司主账号" />
        </el-form-item>
        <el-form-item label="厂商" required>
          <el-select v-model="form.provider" style="width: 100%">
            <el-option v-for="p in connectors" :key="p" :value="p" :label="providerLabel(p)" />
          </el-select>
        </el-form-item>
        <el-form-item label="AccessKey" required>
          <el-input
            v-model="form.access_key" :placeholder="akPlaceholder" show-password
          />
        </el-form-item>
        <el-form-item label="SecretKey" :required="form.provider !== 'cloudflare'">
          <el-input
            v-model="form.secret_key" :placeholder="skPlaceholder" show-password
          />
        </el-form-item>
        <el-form-item v-if="form.provider === 'aws'" label="Region">
          <el-input v-model="form.region" placeholder="默认 us-east-1" />
        </el-form-item>
        <el-form-item v-if="form.provider === 'cloudflare'" label="Account ID">
          <el-input v-model="form.region" placeholder="可选；填写后同步 Registrar 注册域名" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doCreate">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listAccounts, createAccount, deleteAccount, checkAccount, syncAccount } from '../api/domhub'

const loading = ref(false)
const saving = ref(false)
const accounts = ref([])
const connectors = ref(['tencent', 'aliyun', 'aws'])
const dialogVisible = ref(false)
const form = ref({ name: '', provider: 'tencent', access_key: '', secret_key: '', region: '' })

const providerLabels = {
  tencent: '腾讯云',
  aliyun: '阿里云',
  aws: 'AWS',
  cloudflare: 'Cloudflare',
}

const providerLabel = (p) => providerLabels[p] || p
const providerTagType = (p) => ({ tencent: 'primary', aliyun: 'warning', aws: 'warning', cloudflare: 'danger' }[p] || 'info')

const akPlaceholder = computed(() =>
  form.value.provider === 'cloudflare'
    ? 'API Token（推荐）或 Account Email（搭配 Global API Key）'
    : 'AccessKey ID')
const skPlaceholder = computed(() =>
  form.value.provider === 'cloudflare'
    ? '留空使用 API Token；或填 Global API Key（搭配 Email）'
    : 'AccessKey Secret（AES 加密存储）')

const formatTime = (t) => (t ? new Date(t).toLocaleString('zh-CN') : '—')

async function load() {
  loading.value = true
  try {
    const res = await listAccounts()
    accounts.value = res.data.items
    if (res.data.connectors?.length) connectors.value = res.data.connectors
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { name: '', provider: connectors.value[0] || 'tencent', access_key: '', secret_key: '', region: '' }
  dialogVisible.value = true
}

async function doCreate() {
  saving.value = true
  try {
    await createAccount(form.value)
    ElMessage.success('账号已接入')
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function doCheck(row) {
  const res = await checkAccount(row.id)
  if (res.data.last_check_ok) {
    ElMessage.success('连接正常')
  } else {
    ElMessage.error(`连接异常：${res.data.last_check_msg}`)
  }
  await load()
}

async function doSync(row) {
  const res = await syncAccount(row.id)
  if (res.data.status === 'success') {
    ElMessage.success(`同步完成，共 ${res.data.domain_count} 条`)
  } else {
    ElMessage.warning(`同步失败：${res.data.message}`)
  }
  await load()
}

async function doDelete(row) {
  await ElMessageBox.confirm(`确定删除账号「${row.name}」？其域名台账数据将一并删除。`, '删除确认', { type: 'warning' })
  await deleteAccount(row.id)
  ElMessage.success('已删除')
  await load()
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
.conn-tag {
  margin-left: 8px;
}
.check-msg {
  margin-left: 8px;
  font-size: 12px;
  color: #909399;
}
.text-muted {
  color: #c0c4cc;
  font-size: 12px;
}
</style>
