<template>
  <div>
    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <span>SSL 证书监控</span>
          <div>
            <el-input
              v-model="newHost" placeholder="手动添加主机，如 api.example.com" size="small"
              style="width: 240px; margin-right: 8px" @keyup.enter="doAddHost"
            />
            <el-button size="small" @click="doAddHost">添加</el-button>
            <el-button size="small" type="primary" :loading="certChecking" @click="doCertCheck">立即检查</el-button>
          </div>
        </div>
      </template>
      <div class="cert-toolbar">
        <div class="cert-stats">
          <span>共 {{ certStats.total }} 个主机</span>
          <el-tag size="small" type="success" effect="plain">健康 {{ certStats.healthy }}</el-tag>
          <el-tag size="small" type="warning" effect="plain">待续期 {{ certStats.expiring }}</el-tag>
          <el-tag size="small" type="danger" effect="plain">异常 {{ certStats.errors }}</el-tag>
          <el-tag size="small" type="info" effect="plain">未检测 {{ certStats.unchecked }}</el-tag>
        </div>
        <el-switch
          v-model="certAttentionOnly" size="small"
          active-text="仅看需关注" inactive-text="显示全部"
        />
      </div>
      <div class="cert-summary">监控对象为各主域及其解析记录（A/AAAA/CNAME）中的子域名，镜像每日校准、证书每天 08:00 自动检查</div>
      <el-empty
        v-if="!certs.length && !loading" description="尚未检查，点击「立即检查」自动发现并探测各主机名的 HTTPS 证书"
        :image-size="60"
      />
      <el-empty
        v-else-if="!displayCerts.length" description="没有需要关注的证书，全部健康"
        :image-size="60"
      />
      <el-table v-else v-loading="certChecking" :data="displayCerts" stripe size="small">
        <el-table-column prop="host" label="主机" min-width="200" />
        <el-table-column label="来源" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.source === 'manual' ? 'warning' : 'info'" effect="plain">
              {{ row.source === 'manual' ? '手动' : '自动' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="证书到期" width="120">
          <template #default="{ row }">
            {{ row.not_after ? row.not_after.slice(0, 10) : '—' }}
          </template>
        </el-table-column>
        <el-table-column label="剩余天数" width="110" align="center">
          <template #default="{ row }">
            <template v-if="row.excluded">
              <el-tag size="small" type="info">已排除</el-tag>
            </template>
            <el-tag v-else-if="row.ok" size="small" :type="certTagType(row.days_left)">
              {{ row.days_left }} 天
            </el-tag>
            <el-tooltip v-else :content="row.error" placement="top">
              <el-tag size="small" type="info">未检测</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="issuer" label="签发者" min-width="130" show-overflow-tooltip />
        <el-table-column label="检查时间" width="160">
          <template #default="{ row }">
            <span class="cert-checked">{{ formatTime(row.checked_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="doToggleExcluded(row)">{{ row.excluded ? '取消排除' : '排除' }}</el-button>
            <el-button link size="small" type="danger" @click="doDeleteCert(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCerts, runCertCheck, addCertHost, deleteCert, setCertExcluded } from '../api/domhub'

const loading = ref(false)
const certs = ref([])
const certChecking = ref(false)
const newHost = ref('')

// 证书卡片默认只展示需关注项（即将到期或异常），健康的只计入总数
const certAttentionOnly = ref(true)

// 从未探测成功的主机（如不提供 443 服务的解析记录）不算异常、不进"需关注"，
// 只计入"未检测"，避免刷屏；曾拿到过证书、现在探测失败的才视为异常。
const hadCert = (c) => !!c.not_after
const certStats = computed(() => {
  const active = certs.value.filter((c) => !c.excluded)
  return {
    total: active.length,
    healthy: active.filter((c) => c.ok && c.days_left > 30).length,
    expiring: active.filter((c) => c.ok && c.days_left <= 30).length,
    errors: active.filter((c) => !c.ok && hadCert(c)).length,
    unchecked: active.filter((c) => !c.ok && !hadCert(c)).length,
  }
})
const displayCerts = computed(() => {
  const active = certs.value.filter((c) => !c.excluded)
  if (!certAttentionOnly.value) return active
  return active.filter((c) => (c.ok && c.days_left <= 30) || (!c.ok && hadCert(c)))
})

const certTagType = (days) => (days <= 7 ? 'danger' : days <= 30 ? 'warning' : 'success')
const formatTime = (t) => (t ? new Date(t).toLocaleString('zh-CN') : '—')

async function load() {
  loading.value = true
  try {
    const cs = await listCerts()
    certs.value = cs.data.items
  } finally {
    loading.value = false
  }
}

async function doCertCheck() {
  certChecking.value = true
  try {
    const res = await runCertCheck()
    ElMessage.success(`已检查 ${res.data.checked} 个主机，发送 ${res.data.alerts_sent} 条告警`)
    const cs = await listCerts()
    certs.value = cs.data.items
  } catch {
    // 拦截器已弹出错误提示
  } finally {
    certChecking.value = false
  }
}

async function doAddHost() {
  const host = newHost.value.trim()
  if (!host) {
    ElMessage.warning('请输入要监控的主机名')
    return
  }
  try {
    await addCertHost({ host })
    newHost.value = ''
    ElMessage.success('已添加，点击「立即检查」获取证书状态')
    const cs = await listCerts()
    certs.value = cs.data.items
  } catch {
    // 拦截器已弹出错误提示
  }
}

async function doToggleExcluded(row) {
  try {
    await setCertExcluded(row.id, !row.excluded)
    const cs = await listCerts()
    certs.value = cs.data.items
  } catch {
    // 拦截器已弹出错误提示
  }
}

async function doDeleteCert(row) {
  try {
    await ElMessageBox.confirm(
      `确定停止监控 ${row.host} 的证书？`,
      '删除监控条目',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  try {
    await deleteCert(row.id)
    const cs = await listCerts()
    certs.value = cs.data.items
  } catch {
    // 拦截器已弹出错误提示
  }
}

onMounted(load)
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
.cert-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.cert-stats {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.cert-checked {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.cert-summary {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 10px;
}
</style>
