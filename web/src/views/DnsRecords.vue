<template>
  <div class="dns-records-page">
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <el-button :icon="Back" @click="goBack">返回列表</el-button>
        <span class="zone-title">{{ zoneName }}</span>
        <el-tag v-if="accountName" size="small" effect="plain" type="info">
          {{ accountName }} · {{ providerLabel(provider) }}
        </el-tag>
        <el-button :icon="Refresh" :loading="recordsLoading" @click="loadRecords">刷新</el-button>
        <div class="spacer" />
        <el-button type="primary" :icon="Plus" @click="openCreate">添加记录</el-button>
        <el-button type="success" plain :disabled="!changed" @click="openPlanDialog">
          预览变更{{ planCount ? `（${planCount}）` : '' }}
        </el-button>
        <el-button @click="openSnapshots">快照</el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <div class="record-filters">
        <el-input
          v-model="nameFilter" placeholder="筛选主机记录" style="width: 200px" clearable
          :prefix-icon="Search" size="small"
        />
        <el-select v-model="typeFilter" placeholder="全部类型" style="width: 130px" size="small" clearable>
          <el-option v-for="t in recordTypes" :key="t" :value="t" :label="t" />
        </el-select>
        <span class="filter-count">共 {{ filteredRecords.length }} 条记录</span>
      </div>
      <el-table :data="filteredRecords" v-loading="recordsLoading" stripe>
        <el-table-column label="主机记录" prop="name" width="150" sortable>
          <template #default="{ row }">
            <el-tag v-if="row.name === '@'" size="small" type="info">@</el-tag>
            <span v-else>{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="类型" prop="type" width="100" sortable>
          <template #default="{ row }">
            <el-tag size="small" :type="typeTag(row.type)">{{ row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="记录值" min-width="260">
          <template #default="{ row }">
            <span class="record-value">{{ row.value }}</span>
          </template>
        </el-table-column>
        <el-table-column label="TTL" prop="ttl" width="90" sortable />
        <el-table-column label="线路" prop="line" width="90" />
        <el-table-column label="优先级" prop="priority" width="80">
          <template #default="{ row }">{{ row.priority || '—' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="!row.id" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" :disabled="!row.id" @click="removeRecord(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="该域名下暂无解析记录" />
        </template>
      </el-table>
    </el-card>

    <!-- 添加/编辑记录 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑解析记录' : '添加解析记录'" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="主机记录" required>
          <el-input v-model="form.name" placeholder="@ 表示根域名，如 www" />
        </el-form-item>
        <el-form-item label="记录类型" required>
          <el-select v-model="form.type" style="width: 100%">
            <el-option v-for="t in recordTypes" :key="t" :value="t" :label="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="记录值" required>
          <el-input
            v-model="form.value" type="textarea" :rows="2"
            :placeholder="form.type === 'MX' ? '如 mail.example.com（多条值用换行分隔）' : '多条值用换行分隔'"
          />
        </el-form-item>
        <el-form-item v-if="['MX', 'SRV'].includes(form.type)" label="优先级">
          <el-input-number v-model="form.priority" :min="0" :max="65535" />
        </el-form-item>
        <el-form-item label="TTL（秒）">
          <el-select v-model="form.ttl" style="width: 100%">
            <el-option v-for="t in ttlOptions" :key="t" :value="t" :label="t" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="isCNProvider" label="线路">
          <el-select v-model="form.line" style="width: 100%">
            <el-option label="默认" value="default" />
            <el-option label="境内（阿里/腾讯通用默认）" value="default" hidden />
            <el-option label="联通" value="联通" />
            <el-option label="电信" value="电信" />
            <el-option label="移动" value="移动" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveRecord">确定</el-button>
      </template>
    </el-dialog>

    <!-- 变更预览 / 执行 -->
    <el-dialog v-model="planDialogVisible" title="变更预览" width="680px">
      <el-alert v-if="planActions.length" type="warning" :closable="false" show-icon
        :title="`共 ${planActions.length} 项变更，确认后将对现网 DNS 执行以下操作`" class="plan-alert" />
      <el-table :data="planActions" size="small" max-height="380">
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.action === 'create' ? 'success' : row.action === 'delete' ? 'danger' : 'warning'">
              {{ { create: '新增', update: '修改', delete: '删除' }[row.action] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="主机记录" width="110">
          <template #default="{ row }">{{ row.record.name }}</template>
        </el-table-column>
        <el-table-column label="类型" width="80">
          <template #default="{ row }">{{ row.record.type }}</template>
        </el-table-column>
        <el-table-column label="记录值" min-width="220">
          <template #default="{ row }">
            <span class="record-value">{{ row.record.value }}</span>
          </template>
        </el-table-column>
        <el-table-column label="TTL" width="80">
          <template #default="{ row }">{{ row.record.ttl }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!planActions.length && !planLoading" description="没有检测到变更" :image-size="60" />
      <template #footer>
        <el-button @click="planDialogVisible = false">关闭</el-button>
        <el-button type="danger" :loading="pushing" :disabled="!planActions.length" @click="execPush">
          执行变更
        </el-button>
      </template>
    </el-dialog>

    <!-- 快照抽屉 -->
    <el-drawer v-model="snapshotDrawer" :title="`解析快照 · ${zoneName}`" size="62%">
      <div class="snap-toolbar">
        <el-button type="primary" size="small" :loading="snapCapturing" @click="captureNow">保存当前快照</el-button>
        <el-button size="small" :disabled="snapSelection.length !== 2" @click="diffSelected">
          比较选中两份{{ snapSelection.length === 2 ? `（${snapDiffLabel}）` : '' }}
        </el-button>
        <span class="snap-hint">查看记录时会自动保存快照；每 Zone 最多保留 50 份</span>
      </div>
      <el-table :data="snapshots" size="small" @selection-change="(v) => (snapSelection = v)">
        <el-table-column type="selection" width="42" />
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ (row.created_at || '').replace('T', ' ').slice(0, 19) }}</template>
        </el-table-column>
        <el-table-column label="来源" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="{ view: 'info', manual: 'success', drift: 'danger' }[row.source] || 'info'">
              {{ { view: '查看', manual: '手动', drift: '漂移' }[row.source] || row.source }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="记录数" prop="count" width="80" />
        <el-table-column label="备注" prop="note" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" size="small" @click="restoreFrom(row)">恢复此快照</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-dialog v-model="snapDiffVisible" title="快照比较（基准 → 目标）" width="640px" append-to-body>
        <el-alert v-if="snapDiffPlan.length" type="warning" :closable="false" show-icon
          :title="`两份快照间共 ${snapDiffPlan.length} 处差异`" />
        <el-empty v-else description="两份快照内容一致" :image-size="60" />
        <el-table v-if="snapDiffPlan.length" :data="snapDiffPlan" size="small" max-height="360">
          <el-table-column label="操作" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="row.action === 'create' ? 'success' : row.action === 'delete' ? 'danger' : 'warning'">
                {{ { create: '新增', update: '修改', delete: '删除' }[row.action] }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="主机记录" prop="record.name" width="110" />
          <el-table-column label="类型" prop="record.type" width="70" />
          <el-table-column label="记录值" min-width="220">
            <template #default="{ row }"><span class="record-value">{{ row.record.value }}</span></template>
          </el-table-column>
        </el-table>
      </el-dialog>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Back, Plus, Refresh, Search } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import {
  listAccounts, listDNSRecords,
  createDNSRecord, updateDNSRecord, deleteDNSRecord,
  planDNS, pushDNS,
  listSnapshots, captureSnapshot, diffSnapshots, restorePlan,
} from '../api/domhub'

const route = useRoute()
const router = useRouter()

// Zone 上下文来自路由：/dns/records?account_id=1&zone=example.com[&snapshot=1]
const accountId = ref(Number(route.query.account_id) || 0)
const zoneName = ref(String(route.query.zone || ''))

const accountName = ref('')
const provider = ref('')

const recordsLoading = ref(false)
const records = ref([])
const snapshot = ref([]) // 现网快照，用于本地变更计数

const nameFilter = ref('')
const typeFilter = ref('')
const filteredRecords = computed(() => {
  const kw = nameFilter.value.trim().toLowerCase()
  return records.value
    .filter((r) => !kw || r.name.toLowerCase().includes(kw))
    .filter((r) => !typeFilter.value || r.type === typeFilter.value)
})

const dialogVisible = ref(false)
const editing = ref(false)
const saving = ref(false)
const form = ref({})

const planDialogVisible = ref(false)
const planLoading = ref(false)
const planActions = ref([])
const pushing = ref(false)

const snapshotDrawer = ref(false)
const snapshots = ref([])
const snapCapturing = ref(false)
const snapSelection = ref([])
const snapDiffVisible = ref(false)
const snapDiffPlan = ref([])
const snapDiffLabel = ref('')

const recordTypes = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'CAA', 'SRV']
const ttlOptions = [60, 300, 600, 900, 1800, 3600, 7200, 86400]

const providerLabel = (p) => ({ aliyun: '阿里云', tencent: '腾讯云', aws: 'AWS' }[p] || p)
const isCNProvider = computed(() => ['aliyun', 'tencent'].includes(provider.value))

const planCount = computed(() => {
  // 简化：本地统计与快照不一致的行数，仅作为按钮提示；准确计划由后端 diff 生成
  return records.value.filter((r, i) => {
    const s = snapshot.value[i]
    return !s || s.name !== r.name || s.type !== r.type || s.value !== r.value ||
      s.ttl !== r.ttl || s.priority !== r.priority || s.line !== r.line
  }).length
})
const changed = computed(() => planCount.value > 0)

const typeTag = (t) =>
  ({ A: 'success', AAAA: 'success', CNAME: 'warning', TXT: 'info', MX: 'danger', NS: 'warning' }[t] || 'info')

onMounted(async () => {
  if (!accountId.value || !zoneName.value) {
    ElMessage.error('缺少账号或域名参数')
    return
  }
  // 拉账号信息用于展示归属与判断线路选项（阿里/腾讯才有线路）
  try {
    const res = await listAccounts()
    const a = (res.data?.items || []).find((x) => x.id === accountId.value)
    if (a) {
      accountName.value = a.name
      provider.value = a.provider
    }
  } catch { /* 归属展示失败不阻塞主流程 */ }
  await loadRecords()
  if (route.query.snapshot === '1') {
    await openSnapshots()
  }
})

async function loadRecords() {
  recordsLoading.value = true
  try {
    const res = await listDNSRecords(accountId.value, zoneName.value)
    records.value = res.data || []
    snapshot.value = records.value.map((r) => ({ ...r }))
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取解析记录失败')
  } finally {
    recordsLoading.value = false
  }
}

function openCreate() {
  editing.value = false
  form.value = { name: '', type: 'A', value: '', ttl: 600, priority: 0, line: 'default' }
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = true
  form.value = {
    record_id: row.id, name: row.name, type: row.type, value: row.value,
    ttl: row.ttl || 600, priority: row.priority || 0, line: row.line || 'default',
  }
  dialogVisible.value = true
}

async function saveRecord() {
  const f = form.value
  if (!f.name || !f.type || !f.value) {
    ElMessage.warning('主机记录、类型与记录值不能为空')
    return
  }
  saving.value = true
  try {
    const payload = {
      account_id: accountId.value, zone: zoneName.value,
      name: f.name, type: f.type, value: f.value,
      ttl: f.ttl, priority: f.priority, line: f.line,
    }
    if (editing.value) {
      payload.record_id = f.record_id
      await updateDNSRecord(payload)
      ElMessage.success('记录已更新')
    } else {
      await createDNSRecord(payload)
      ElMessage.success('记录已创建')
    }
    dialogVisible.value = false
    await loadRecords()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function removeRecord(row) {
  try {
    if (row.type === 'NS') {
      // NS 记录影响域名解析托管权，删除需输入域名强确认
      await ElMessageBox.prompt(
        `即将删除 NS 记录 ${row.name} → ${row.value.slice(0, 50)}。删除可能导致域名失去解析托管，影响线上服务！请输入完整域名 ${zoneName.value} 确认。`,
        '删除 NS 记录（高危）',
        { type: 'error', confirmButtonText: '确认删除', cancelButtonText: '取消',
          inputPattern: new RegExp(`^${zoneName.value.replace(/\./g, '\\.')}$`),
          inputErrorMessage: `请输入完整域名 ${zoneName.value}` },
      )
    } else {
      await ElMessageBox.confirm(
        `确认删除记录 ${row.type} ${row.name} → ${row.value.slice(0, 50)}？此操作立即生效且不可撤销。`,
        '删除解析记录',
        { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
      )
    }
  } catch {
    return
  }
  try {
    await deleteDNSRecord({
      account_id: accountId.value, zone: zoneName.value,
      record_id: row.id, desc: `${row.type} ${row.name}`,
    })
    ElMessage.success('记录已删除')
    await loadRecords()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '删除失败')
  }
}

async function openPlanDialog() {
  planDialogVisible.value = true
  planLoading.value = true
  try {
    const res = await planDNS({
      account_id: accountId.value,
      zone: zoneName.value,
      desired: records.value.map((r) => ({
        id: r.id || '', name: r.name, type: r.type, value: r.value,
        ttl: r.ttl || 600, priority: r.priority || 0, line: r.line || '',
      })),
    })
    planActions.value = res.data || []
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '生成变更计划失败')
    planDialogVisible.value = false
  } finally {
    planLoading.value = false
  }
}

async function execPush() {
  const nsActions = planActions.value.filter(
    (a) => a.record?.type === 'NS' && ['delete', 'update'].includes(a.action),
  )
  try {
    if (nsActions.length) {
      // 变更中包含 NS 的删除/修改，需要输入域名强确认
      await ElMessageBox.prompt(
        `本次变更包含 ${nsActions.length} 项 NS 记录的删除/修改，可能导致域名失去解析托管！请输入完整域名 ${zoneName.value} 确认执行。`,
        '执行变更（含高危 NS 操作）',
        { type: 'error', confirmButtonText: '确认执行', cancelButtonText: '取消',
          inputPattern: new RegExp(`^${zoneName.value.replace(/\./g, '\\.')}$`),
          inputErrorMessage: `请输入完整域名 ${zoneName.value}` },
      )
    } else {
      await ElMessageBox.confirm(
        `确认对 ${zoneName.value} 执行 ${planActions.value.length} 项 DNS 变更？此操作直接影响线上解析。`,
        '执行变更',
        { type: 'warning', confirmButtonText: '执行', cancelButtonText: '取消' },
      )
    }
  } catch {
    return
  }
  pushing.value = true
  try {
    const res = await pushDNS({
      account_id: accountId.value,
      zone: zoneName.value,
      actions: planActions.value,
    })
    const results = res.data || []
    const failed = results.filter((r) => !r.success)
    if (failed.length) {
      ElMessage.error(`${results.length - failed.length}/${results.length} 成功，${failed.length} 失败，详见审计日志`)
    } else {
      ElMessage.success(`全部 ${results.length} 项变更执行成功`)
    }
    planDialogVisible.value = false
    await loadRecords()
    // 变更后自动留存快照，作为漂移检测的新基线
    try {
      await captureSnapshot({ account_id: accountId.value, zone: zoneName.value, note: '变更执行后自动快照' })
    } catch { /* 快照失败不影响主流程 */ }
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '执行失败')
  } finally {
    pushing.value = false
  }
}

// 返回列表并通知刷新该账号的记录数缓存
function goBack() {
  router.push({ path: '/dns', query: { refresh_account: accountId.value } })
}

// ---- 快照 ----
async function openSnapshots() {
  snapshotDrawer.value = true
  await loadSnapshots()
}

async function loadSnapshots() {
  try {
    const res = await listSnapshots(accountId.value, zoneName.value)
    snapshots.value = res.data || []
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取快照失败')
  }
}

async function captureNow() {
  snapCapturing.value = true
  try {
    await captureSnapshot({ account_id: accountId.value, zone: zoneName.value, note: '手动快照' })
    ElMessage.success('快照已保存')
    await loadSnapshots()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存快照失败')
  } finally {
    snapCapturing.value = false
  }
}

async function diffSelected() {
  const sel = [...snapSelection.value].sort((a, b) => a.id - b.id)
  const [base, target] = sel
  snapDiffLabel.value = `${(base.created_at || '').slice(5, 16)} → ${(target.created_at || '').slice(5, 16)}`
  try {
    const res = await diffSnapshots(base.id, target.id)
    snapDiffPlan.value = res.data?.plan || []
    snapDiffVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '比较失败')
  }
}

async function restoreFrom(row) {
  try {
    await ElMessageBox.confirm(
      `将以 ${(row.created_at || '').replace('T', ' ').slice(0, 19)} 的快照（${row.count} 条记录）为目标生成恢复计划。现网与快照的差异将生成对应的增/改/删操作，确认后才执行。`,
      '从快照恢复',
      { type: 'warning', confirmButtonText: '生成恢复计划', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  try {
    const res = await restorePlan({ account_id: accountId.value, zone: zoneName.value, snapshot_id: row.id })
    planActions.value = res.data || []
    snapshotDrawer.value = false
    if (!planActions.value.length) {
      ElMessage.info('现网与该快照一致，无需恢复')
      return
    }
    planDialogVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '生成恢复计划失败')
  }
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
.zone-title {
  font-size: 16px;
  font-weight: 600;
}
.record-value {
  word-break: break-all;
  white-space: pre-wrap;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
}
.record-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.filter-count {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.plan-alert {
  margin-bottom: 12px;
}
.snap-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.snap-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
