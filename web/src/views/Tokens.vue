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
      <template #header><span>调用示例</span></template>
      <pre class="example">curl -H "Authorization: Bearer dht_你的令牌" \
  {{ apiBase }}/domains</pre>
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
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listTokens, createToken, revokeToken } from '../api/domhub'
import http from '../api/http'

const tokens = ref([])
const loading = ref(false)
const createVisible = ref(false)
const creating = ref(false)
const created = ref('')
const form = ref({ name: '', expire_days: 0 })
const apiBase = `${window.location.origin}/api/v1`

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
