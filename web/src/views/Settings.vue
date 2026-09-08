<template>
  <div>
    <el-card shadow="never" class="toolbar-card">
      <template #header><b>定时任务</b></template>
      <el-form label-width="180px" class="settings-form">
        <el-form-item label="域名到期检查">
          <div class="cron-row">
            <el-input v-model="form.expiry_check_cron" placeholder="0 0 9 * * *" style="width: 220px" />
            <el-switch v-model="expiryEnabled" active-text="启用" />
          </div>
          <div class="field-hint">默认每天 09:00 检查所有注册域名到期时间并按规则告警。示例：每天 9 点 `0 0 9 * * *`、每小时 `0 0 * * * *`</div>
        </el-form-item>
        <el-form-item label="域名台账自动同步">
          <div class="cron-row">
            <el-input v-model="form.sync_domains_cron" placeholder="0 0 6 * * *" style="width: 220px" />
            <el-switch v-model="syncEnabled" active-text="启用" />
          </div>
          <div class="field-hint">定时从各云厂商同步域名台账。默认关闭（同步也可在台账页手动触发）</div>
        </el-form-item>
        <el-form-item label="DNS 漂移检测">
          <div class="cron-row">
            <el-input v-model="form.drift_check_cron" placeholder="0 0 10 * * *" style="width: 220px" />
            <el-switch v-model="driftEnabled" active-text="启用" />
          </div>
          <div class="field-hint">比对现网解析与最近快照，发现偏离（如有人在云控制台直接改了解析）时通过告警渠道通知，并自动留存新快照。默认关闭；需要先在 DNS 管理页为 Zone 保存过快照</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存并生效</el-button>
          <span class="field-hint save-hint">保存后立即热生效，无需重启服务</span>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never">
      <template #header><b>系统信息</b></template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="版本">{{ info.version || '—' }}</el-descriptions-item>
        <el-descriptions-item label="Go 版本">{{ info.go_version || '—' }}</el-descriptions-item>
        <el-descriptions-item label="启动时间">{{ formatTime(info.started_at) }}</el-descriptions-item>
        <el-descriptions-item label="数据库">{{ info.db_engine || '—' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getSettings, updateSchedules } from '../api/domhub'

const saving = ref(false)
const form = reactive({ expiry_check_cron: '-', sync_domains_cron: '-', drift_check_cron: '-' })
const info = ref({})

const expiryEnabled = computed({
  get: () => form.expiry_check_cron !== '-',
  set: (v) => { form.expiry_check_cron = v ? '0 0 9 * * *' : '-' },
})
const syncEnabled = computed({
  get: () => form.sync_domains_cron !== '-',
  set: (v) => { form.sync_domains_cron = v ? '0 0 6 * * *' : '-' },
})
const driftEnabled = computed({
  get: () => form.drift_check_cron !== '-',
  set: (v) => { form.drift_check_cron = v ? '0 0 10 * * *' : '-' },
})

const formatTime = (t) => (t ? new Date(t).toLocaleString('zh-CN') : '—')

async function load() {
  try {
    const res = await getSettings()
    Object.assign(form, res.data?.schedules || {})
    info.value = res.data?.info || {}
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '加载设置失败')
  }
}

async function save() {
  saving.value = true
  try {
    await updateSchedules({ ...form })
    ElMessage.success('已保存并生效')
    await load()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar-card {
  margin-bottom: 16px;
}
.settings-form {
  max-width: 720px;
}
.cron-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.field-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
  margin-top: 4px;
}
.save-hint {
  margin-left: 12px;
}
</style>
