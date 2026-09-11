<template>
  <div>
    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <span>告警渠道</span>
          <el-button size="small" type="primary" @click="openChannelDialog()">新增渠道</el-button>
        </div>
      </template>
      <el-table v-loading="loading" :data="channels" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag size="small">{{ channelLabels[row.type] || row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '是' : '否' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" :loading="testingId === row.id" @click="doTestChannel(row)">测试</el-button>
            <el-button size="small" @click="openChannelDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" plain @click="doDeleteChannel(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <span>告警规则</span>
          <div>
            <el-button size="small" :loading="checking" @click="doRunCheck">立即检查</el-button>
            <el-button size="small" type="primary" @click="openRuleDialog()">新增规则</el-button>
          </div>
        </div>
      </template>
      <el-table v-loading="loading" :data="rules" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="row.kind === 'cert_expire' ? 'warning' : 'primary'" effect="plain">
              {{ kindLabels[row.kind] || row.kind }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="offsets" label="提前天数" width="160" />
        <el-table-column label="通知渠道" min-width="160">
          <template #default="{ row }">
            <el-tag v-for="cid in parseChannelIDs(row.channel_ids)" :key="cid" size="small" class="tag-item">
              {{ channelName(cid) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="(v) => toggleRule(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-button size="small" @click="openRuleDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" plain @click="doDeleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header"><span>最近告警记录</span></div>
      </template>
      <el-empty v-if="!logs.length" description="暂无告警记录" :image-size="60" />
      <el-timeline v-else>
        <el-timeline-item v-for="log in logs" :key="log.id" :timestamp="formatTime(log.sent_at)">
          <b>{{ log.domain_name }}</b>（提前 {{ log.offset }} 天档）— {{ log.message }}
        </el-timeline-item>
      </el-timeline>
    </el-card>

    <!-- 渠道编辑 -->
    <el-dialog v-model="channelDialog" :title="channelForm.id ? '编辑渠道' : '新增渠道'" width="520">
      <el-form :model="channelForm" label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="channelForm.name" placeholder="如：运维钉钉群" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-select v-model="channelForm.type" style="width: 100%" @change="channelForm.config = ''">
            <el-option v-for="(label, key) in channelLabels" :key="key" :value="key" :label="label" />
          </el-select>
        </el-form-item>
        <el-form-item label="渠道配置" required>
          <el-input
            v-model="channelForm.config"
            type="textarea" :rows="3"
            :placeholder="configPlaceholder"
          />
          <div class="config-hint">{{ configHint }}</div>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="channelForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelDialog = false">取消</el-button>
        <el-button :loading="testingForm" @click="doTestChannelForm">发送测试</el-button>
        <el-button type="primary" @click="saveChannel">保存</el-button>
      </template>
    </el-dialog>

    <!-- 规则编辑 -->
    <el-dialog v-model="ruleDialog" :title="ruleForm.id ? '编辑规则' : '新增规则'" width="520">
      <el-form :model="ruleForm" label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="ruleForm.name" placeholder="如：域名到期提醒" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-radio-group v-model="ruleForm.kind">
            <el-radio value="domain_expire">域名到期</el-radio>
            <el-radio value="cert_expire">SSL 证书到期</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="提前天数" required>
          <el-checkbox-group v-model="ruleForm.offsetList">
            <el-checkbox :value="60">60 天</el-checkbox>
            <el-checkbox :value="30">30 天</el-checkbox>
            <el-checkbox :value="7">7 天</el-checkbox>
            <el-checkbox :value="1">1 天</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="通知渠道" required>
          <el-select v-model="ruleForm.channelIdList" multiple style="width: 100%" placeholder="选择渠道">
            <el-option v-for="ch in channels" :key="ch.id" :value="ch.id" :label="ch.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="ruleForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRule">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listChannels, createChannel, updateChannel, deleteChannel,
  testChannel, testChannelByID,
  listRules, createRule, updateRule, deleteRule,
  runAlertCheck, listAlertLogs,
} from '../api/domhub'

const channelLabels = {
  webhook: 'Webhook',
  dingtalk: '钉钉机器人',
  wecom: '企业微信机器人',
  email: '邮件',
  telegram: 'Telegram',
}

const kindLabels = { domain_expire: '域名到期', cert_expire: '证书到期' }

const loading = ref(false)
const checking = ref(false)
const channels = ref([])
const rules = ref([])
const logs = ref([])

const channelDialog = ref(false)
const channelForm = ref({ id: 0, name: '', type: 'webhook', config: '', enabled: true })
const testingId = ref(0)
const testingForm = ref(false)

const ruleDialog = ref(false)
const ruleForm = ref({ id: 0, name: '', kind: 'domain_expire', offsetList: [60, 30, 7, 1], channelIdList: [], enabled: true })

const configPlaceholder = computed(() => ({
  webhook: 'https://your-server.com/hook（接收 {"title","content"} JSON）',
  dingtalk: '钉钉机器人 Webhook 地址',
  wecom: '企业微信群机器人 Webhook 地址',
  email: '{"host":"smtp.qq.com","port":465,"username":"a@b.com","password":"xx","from":"a@b.com","to":["me@x.com"]}',
  telegram: '{"bot_token":"123:abc","chat_id":"456"}',
}[channelForm.value.type] || '{}'))

const configHint = computed(() =>
  ['email', 'telegram'].includes(channelForm.value.type) ? '配置为 JSON 格式（见占位提示）' : '直接填写 Webhook 地址即可'
)

const channelName = (id) => channels.value.find((c) => c.id === id)?.name || `#${id}`
const parseChannelIDs = (s) => {
  try { return JSON.parse(s) || [] } catch { return [] }
}
const formatTime = (t) => (t ? new Date(t).toLocaleString('zh-CN') : '—')

async function load() {
  loading.value = true
  try {
    const [ch, ru, lg] = await Promise.all([
      listChannels(), listRules(), listAlertLogs(30),
    ])
    channels.value = ch.data.items
    rules.value = ru.data.items
    logs.value = lg.data.items
  } finally {
    loading.value = false
  }
}

function openChannelDialog(row) {
  if (row) {
    channelForm.value = { ...row }
  } else {
    channelForm.value = { id: 0, name: '', type: 'webhook', config: '', enabled: true }
  }
  channelDialog.value = true
}

async function doTestChannel(row) {
  testingId.value = row.id
  try {
    const res = await testChannelByID(row.id)
    res.code === 0 ? ElMessage.success(res.message) : ElMessage.error(res.message)
  } catch {
    // 拦截器已弹出错误提示
  } finally {
    testingId.value = 0
  }
}

async function doTestChannelForm() {
  const f = channelForm.value
  if (!f.config) {
    ElMessage.warning('请先填写渠道配置')
    return
  }
  testingForm.value = true
  try {
    const res = await testChannel({ type: f.type, config: f.config })
    res.code === 0 ? ElMessage.success(res.message) : ElMessage.error(res.message)
  } catch {
    // 拦截器已弹出错误提示
  } finally {
    testingForm.value = false
  }
}

async function saveChannel() {
  const f = channelForm.value
  if (!f.name || !f.config) {
    ElMessage.warning('请完整填写名称与渠道配置')
    return
  }
  if (f.id) {
    await updateChannel(f.id, f)
  } else {
    await createChannel(f)
  }
  ElMessage.success('已保存')
  channelDialog.value = false
  await load()
}

function safeParse(s) {
  try { return JSON.parse(s) } catch { return null }
}
// safeParse 保留供后续渠道配置校验使用
void safeParse

async function doDeleteChannel(row) {
  await ElMessageBox.confirm(`确定删除渠道「${row.name}」？`, '删除确认', { type: 'warning' })
  await deleteChannel(row.id)
  await load()
}

function openRuleDialog(row) {
  if (row) {
    ruleForm.value = {
      id: row.id,
      name: row.name,
      kind: row.kind || 'domain_expire',
      offsetList: (row.offsets || '').split(',').map(Number).filter(Boolean),
      channelIdList: parseChannelIDs(row.channel_ids),
      enabled: row.enabled,
    }
  } else {
    ruleForm.value = { id: 0, name: '', kind: 'domain_expire', offsetList: [60, 30, 7, 1], channelIdList: [], enabled: true }
  }
  ruleDialog.value = true
}

async function saveRule() {
  const f = ruleForm.value
  if (!f.name || !f.offsetList.length || !f.channelIdList.length) {
    ElMessage.warning('请完整填写名称、提前天数与通知渠道')
    return
  }
  const payload = {
    id: f.id,
    name: f.name,
    kind: f.kind || 'domain_expire',
    offsets: [...f.offsetList].sort((a, b) => b - a).join(','),
    channel_ids: JSON.stringify(f.channelIdList),
    enabled: f.enabled,
  }
  if (f.id) {
    await updateRule(f.id, payload)
  } else {
    await createRule(payload)
  }
  ElMessage.success('已保存')
  ruleDialog.value = false
  await load()
}

async function toggleRule(row, v) {
  await updateRule(row.id, { ...row, enabled: v, channel_ids: row.channel_ids })
  await load()
}

async function doDeleteRule(row) {
  await ElMessageBox.confirm(`确定删除规则「${row.name}」？`, '删除确认', { type: 'warning' })
  await deleteRule(row.id)
  await load()
}

async function doRunCheck() {
  checking.value = true
  try {
    const res = await runAlertCheck()
    ElMessage.success(`检查完成，本次发送 ${res.data.alerts_sent} 条告警`)
    await load()
  } finally {
    checking.value = false
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
.tag-item {
  margin-right: 4px;
}
.config-hint {
  font-size: 12px;
  color: #71717a;
  margin-top: 4px;
}
</style>
