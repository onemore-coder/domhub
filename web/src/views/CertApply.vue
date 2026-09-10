<template>
  <div class="cert-apply-page">
    <!-- 申请表单 -->
    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header"><span>申请免费证书（ACME · DNS-01 验证）</span></div>
      </template>
      <el-alert
        type="info" :closable="false" show-icon class="apply-alert"
        title="验证方式为 DNS-01：DomHub 会自动在所选云账号的 DNS 中添加/清理 _acme-challenge TXT 记录，无需改服务器配置，支持泛域名。"
      />
      <el-form :model="form" label-width="110px" class="apply-form">
        <el-form-item label="DNS 云账号" required>
          <el-select v-model="form.dns_account_id" placeholder="选择管理该域名解析的云账号" style="width: 100%">
            <el-option
              v-for="a in accounts" :key="a.id" :value="a.id"
              :label="`${a.name}（${providerLabel(a.provider)}）`"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="主域名" required>
          <el-input v-model="form.primary" placeholder="如 example.com 或 *.example.com（泛域名）" />
        </el-form-item>
        <el-form-item label="附加域名">
          <el-input v-model="form.extra" placeholder="可选，逗号或换行分隔，如 www.example.com,api.example.com" />
        </el-form-item>
        <el-form-item label="联系邮箱" required>
          <el-input v-model="form.email" placeholder="用于 CA 账户注册与到期提醒" />
        </el-form-item>
        <el-form-item label="证书 CA" required>
          <el-select v-model="form.ca" style="width: 100%">
            <el-option v-for="ca in cas" :key="ca.key" :value="ca.key" :label="ca.name" />
          </el-select>
          <div v-if="form.ca === 'letsencrypt_staging'" class="form-hint">
            测试环境证书浏览器不信任，仅用于打通流程；正式使用请选 Let's Encrypt
          </div>
        </el-form-item>
        <el-form-item label="自动续期">
          <el-switch v-model="form.auto_renew" />
          <span class="form-hint inline">到期前 30 天自动续期（每天 08:30 检查）</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="applying" @click="submitApply">提交申请</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 已签发 / 申请记录 -->
    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <span>申请记录</span>
          <el-button size="small" :icon="Refresh" @click="loadList">刷新</el-button>
        </div>
      </template>
      <el-empty v-if="!list.length" description="还没有申请记录，从上方表单发起第一张证书申请" :image-size="70" />
      <el-table v-else :data="list" stripe>
        <el-table-column label="主域名" min-width="180">
          <template #default="{ row }">
            <span class="domain">{{ row.primary_domain }}</span>
          </template>
        </el-table-column>
        <el-table-column label="SAN 数" width="70" align="center">
          <template #default="{ row }">{{ row.sans.split(',').length }}</template>
        </el-table-column>
        <el-table-column label="CA" width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.ca_name }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="statusTag(row.status)" effect="light">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="到期时间 / 剩余" width="180">
          <template #default="{ row }">
            <template v-if="row.not_after">
              <span>{{ row.not_after.slice(0, 10) }}</span>
              <el-tag size="small" :type="daysTag(row)" effect="plain" class="days-tag">
                {{ daysLeft(row) }} 天
              </el-tag>
            </template>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column label="自动续期" width="90" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.auto_renew" size="small"
              @change="(v) => toggleAutoRenew(row, v)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" type="primary" @click="showProgress(row)">进度</el-button>
            <el-button
              link size="small" type="warning" :disabled="!['issued', 'failed'].includes(row.status)"
              @click="renew(row)"
            >续期</el-button>
            <el-dropdown
              v-if="row.status === 'issued'" class="dl-dropdown" size="small"
              @command="(t) => download(row, t)"
            >
              <el-button link size="small" type="success">下载</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="fullchain">证书 + 私钥（fullchain）</el-dropdown-item>
                  <el-dropdown-item command="chain">仅证书链</el-dropdown-item>
                  <el-dropdown-item command="key">仅私钥</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button link size="small" type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 申请进度 -->
    <el-dialog v-model="progressVisible" title="申请进度" width="640px">
      <div v-if="progressCert">
        <div class="progress-head">
          <span class="domain">{{ progressCert.primary_domain }}</span>
          <el-tag size="small" :type="statusTag(progressCert.status)">{{ statusLabel(progressCert.status) }}</el-tag>
          <span v-if="progressCert.last_message" class="last-error">{{ progressCert.last_message }}</span>
        </div>
        <pre class="progress-log">{{ progressCert.progress_log || '（暂无进度）' }}</pre>
        <div v-if="polling" class="polling-hint">正在执行中，每 3 秒自动刷新…</div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { listAccounts, listCertCAs, listIssuedCerts, applyCert, getIssuedCert, renewIssuedCert, setIssuedCertAutoRenew, deleteIssuedCert } from '../api/domhub'

const accounts = ref([])
const cas = ref([])
const list = ref([])
const applying = ref(false)

const form = ref({
  dns_account_id: null, primary: '', extra: '',
  email: '', ca: 'letsencrypt', auto_renew: true,
})

const providerLabel = (p) => ({ aliyun: '阿里云', tencent: '腾讯云', aws: 'AWS', cloudflare: 'Cloudflare' }[p] || p)

const statusLabel = (s) => ({ pending: '排队中', renewing: '执行中', issued: '已签发', failed: '失败' }[s] || s)
const statusTag = (s) => ({ pending: 'info', renewing: 'warning', issued: 'success', failed: 'danger' }[s] || 'info')

const daysLeft = (row) => Math.floor((new Date(row.not_after).getTime() - Date.now()) / 86400000)
const daysTag = (row) => {
  const d = daysLeft(row)
  return d <= 7 ? 'danger' : d <= 30 ? 'warning' : 'success'
}

// 进度轮询
const progressVisible = ref(false)
const progressCert = ref(null)
const polling = ref(false)
let pollTimer = null

onMounted(async () => {
  try {
    const [acc, c] = await Promise.all([listAccounts(), listCertCAs()])
    accounts.value = acc.data?.items || []
    cas.value = c.data || []
  } catch { /* 拦截器已提示 */ }
  await loadList()
})

onBeforeUnmount(stopPoll)

async function loadList() {
  try {
    const res = await listIssuedCerts()
    list.value = res.data?.items || []
  } catch { /* 拦截器已提示 */ }
}

async function submitApply() {
  const f = form.value
  if (!f.dns_account_id) return ElMessage.warning('请选择 DNS 云账号')
  if (!f.primary.trim()) return ElMessage.warning('请输入主域名')
  if (!f.email.trim()) return ElMessage.warning('请输入联系邮箱')
  const domains = [f.primary.trim(), ...f.extra.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean)]
  applying.value = true
  try {
    const res = await applyCert({
      dns_account_id: f.dns_account_id,
      domains,
      email: f.email.trim(),
      ca: f.ca,
      auto_renew: f.auto_renew,
    })
    ElMessage.success(res.message || '申请已提交')
    await loadList()
    showProgress(res.data)
  } catch { /* 拦截器已提示 */ } finally {
    applying.value = false
  }
}

function showProgress(row) {
  progressCert.value = row
  progressVisible.value = true
  startPoll(row.id)
}

function startPoll(id) {
  stopPoll()
  polling.value = ['pending', 'renewing'].includes(progressCert.value?.status)
  if (!polling.value) return
  pollTimer = setInterval(async () => {
    try {
      const res = await getIssuedCert(id)
      progressCert.value = res.data
      const st = res.data?.status
      if (st === 'issued' || st === 'failed') {
        stopPoll()
        loadList()
        if (st === 'issued') ElMessage.success('证书签发成功')
      }
    } catch { stopPoll() }
  }, 3000)
}

function stopPoll() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = null
  polling.value = false
}

async function renew(row) {
  try {
    await renewIssuedCert(row.id)
    ElMessage.success('续期已启动')
    await loadList()
    showProgress({ ...row, status: 'renewing' })
  } catch { /* 拦截器已提示 */ }
}

async function toggleAutoRenew(row, v) {
  try {
    await setIssuedCertAutoRenew(row.id, v)
    row.auto_renew = v
    ElMessage.success(v ? '已开启自动续期' : '已关闭自动续期')
  } catch { /* 拦截器已提示 */ }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(
      `确定删除 ${row.primary_domain} 的申请记录？本地面板将不再保留该证书（已部署到服务器的不受影响）。`,
      '删除申请记录',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
  } catch { return }
  try {
    await deleteIssuedCert(row.id)
    ElMessage.success('已删除')
    await loadList()
  } catch { /* 拦截器已提示 */ }
}

// 下载走 fetch + Bearer 头（浏览器原生 <a> 带不上 Authorization）
async function download(row, type) {
  try {
    const res = await fetch(`/api/v1/certs-issued/${row.id}/download?type=${type}`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('domhub_token')}` },
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    const name = (res.headers.get('Content-Disposition') || '').match(/filename=(.+)/)?.[1] || 'cert.pem'
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.error('下载失败，请稍后重试')
  }
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
.apply-alert {
  margin-bottom: 14px;
}
.apply-form {
  max-width: 560px;
}
.form-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
  line-height: 1.4;
}
.form-hint.inline {
  margin: 0 0 0 10px;
}
.domain {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
}
.days-tag {
  margin-left: 6px;
}
.progress-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.last-error {
  font-size: 12px;
  color: var(--el-color-danger);
}
.progress-log {
  background: var(--el-fill-color-light);
  border-radius: 6px;
  padding: 12px;
  font-size: 12px;
  line-height: 1.7;
  max-height: 320px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
.polling-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 8px;
}
.dl-dropdown {
  margin: 0 10px;
}
</style>
