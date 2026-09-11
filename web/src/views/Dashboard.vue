<template>
  <div v-loading="loading">
    <el-row :gutter="16">
      <el-col v-for="card in cards" :key="card.label" :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-item">
            <div class="stat-icon" :style="{ background: card.color }">
              <el-icon :size="22" color="#fff"><component :is="card.icon" /></el-icon>
            </div>
            <div>
              <div class="stat-value">{{ card.value }}</div>
              <div class="stat-label">{{ card.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <!-- 30 天到期时间线 -->
      <el-col :span="14">
        <el-card shadow="never" class="panel">
          <template #header>
            <div class="panel-header">
              <span>未来 30 天到期分布</span>
              <span class="panel-sub">共 {{ timelineTotal }} 个域名到期</span>
            </div>
          </template>
          <div v-if="timelineTotal" class="timeline-chart">
            <div
              v-for="d in timeline" :key="d.date"
              class="timeline-col"
              :title="`${d.date}：${d.count} 个到期`"
            >
              <div class="bar-wrap">
                <div class="bar" :class="{ urgent: isUrgent(d.date) }" :style="{ height: barHeight(d.count) }" />
              </div>
              <div class="bar-day">{{ dayLabel(d.date) }}</div>
            </div>
          </div>
          <el-empty v-else description="未来 30 天没有域名到期" :image-size="60" />
        </el-card>
      </el-col>

      <!-- 厂商分布 -->
      <el-col :span="10">
        <el-card shadow="never" class="panel">
          <template #header>
            <div class="panel-header"><span>厂商分布</span></div>
          </template>
          <div v-if="providers.length" class="provider-list">
            <div v-for="p in providers" :key="p.provider" class="provider-row">
              <span class="provider-name">{{ providerLabel(p.provider) }}</span>
              <div class="provider-bar-wrap">
                <div class="provider-bar" :style="{ width: providerPct(p.count) + '%', background: providerColor(p.provider) }" />
              </div>
              <span class="provider-count">{{ p.count }}</span>
            </div>
          </div>
          <el-empty v-else description="暂无域名数据" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近变更流 -->
    <el-card shadow="never" class="panel">
      <template #header>
        <div class="panel-header">
          <span>最近变更</span>
          <el-button size="small" link type="primary" @click="$router.push('/audit')">查看全部</el-button>
        </div>
      </template>
      <el-empty v-if="!recent.length" description="暂无操作记录" :image-size="60" />
      <el-timeline v-else class="recent-list">
        <el-timeline-item
          v-for="a in recent" :key="a.id"
          :timestamp="fmtTime(a.created_at)" :type="actionTag(a.action)"
          placement="top"
        >
          <span class="recent-action">{{ actionLabel(a.action) }}</span>
          <span class="recent-user">{{ a.username }}</span>
          <span class="recent-detail">{{ a.resource }}{{ a.detail ? ' · ' + a.detail : '' }}</span>
        </el-timeline-item>
      </el-timeline>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getDashboardSummary, getDashboardStats } from '../api'

const loading = ref(true)
const summary = ref({})
const providers = ref([])
const timeline = ref([])
const recent = ref([])

onMounted(async () => {
  try {
    const [s, st] = await Promise.all([getDashboardSummary(), getDashboardStats()])
    summary.value = s.data || {}
    providers.value = st.data?.providers || []
    timeline.value = st.data?.expiry_timeline || []
    recent.value = st.data?.recent_changes || []
  } finally {
    loading.value = false
  }
})

const cards = computed(() => [
  { label: '域名总数', value: summary.value.domain_total ?? 0, icon: 'Collection', color: '#4f46e5' },
  { label: '托管 Zone', value: summary.value.zone_total ?? 0, icon: 'Connection', color: '#16a34a' },
  { label: '云账号', value: summary.value.account_total ?? 0, icon: 'Cloudy', color: '#d97706' },
  { label: '30 天内到期', value: summary.value.expiring_in_30d ?? 0, icon: 'AlarmClock', color: '#dc2626' },
])

const timelineTotal = computed(() => timeline.value.reduce((n, d) => n + d.count, 0))
const maxCount = computed(() => Math.max(1, ...timeline.value.map((d) => d.count)))
const barHeight = (n) => (n ? Math.max(8, Math.round((n / maxCount.value) * 72)) + 'px' : '2px')
const isUrgent = (date) => (new Date(date).getTime() - Date.now()) / 86400000 <= 7
const dayLabel = (date) => {
  const d = new Date(date)
  const diff = Math.round((d.getTime() - new Date().setHours(0, 0, 0, 0)) / 86400000)
  return diff === 0 ? '今天' : diff === 1 ? '明天' : `${d.getMonth() + 1}/${d.getDate()}`
}

const providerLabel = (p) => ({ aliyun: '阿里云', tencent: '腾讯云', aws: 'AWS', cloudflare: 'Cloudflare' }[p] || p)
const providerColor = (p) => ({ aliyun: '#ff6a00', tencent: '#0052d9', aws: '#ff9900', cloudflare: '#f6821f' }[p] || '#4f46e5')
const maxProvider = computed(() => Math.max(1, ...providers.value.map((p) => p.count)))
const providerPct = (n) => Math.max(4, Math.round((n / maxProvider.value) * 100))

const actionLabel = (a) => ({ create: '新增', update: '修改', delete: '删除', push: '下发', login: '登录', sync: '同步' }[a] || a)
const actionTag = (a) => ({ create: 'success', delete: 'danger', push: 'warning' }[a] || 'info')
const fmtTime = (t) => String(t || '').replace('T', ' ').slice(5, 16)
</script>

<style scoped>
.stat-card {
  margin-bottom: 16px;
}
.stat-item {
  display: flex;
  align-items: center;
  gap: 14px;
}
.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}
.stat-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.panel {
  margin-bottom: 16px;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}
.panel-sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
}
/* 到期时间线 */
.timeline-chart {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 110px;
  padding-top: 6px;
}
.timeline-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
.bar-wrap {
  height: 76px;
  display: flex;
  align-items: flex-end;
}
.bar {
  width: 100%;
  min-width: 6px;
  max-width: 22px;
  border-radius: 3px 3px 0 0;
  background: #4f46e5;
  transition: height 0.3s;
}
.bar.urgent {
  background: #dc2626;
}
.bar-day {
  font-size: 10px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
/* 厂商分布 */
.provider-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 6px 0;
}
.provider-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.provider-name {
  width: 80px;
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
}
.provider-bar-wrap {
  flex: 1;
  height: 14px;
  background: var(--el-fill-color);
  border-radius: 7px;
  overflow: hidden;
}
.provider-bar {
  height: 100%;
  border-radius: 7px;
  transition: width 0.3s;
}
.provider-count {
  width: 32px;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}
/* 最近变更 */
.recent-list {
  padding-left: 4px;
  max-height: 320px;
  overflow-y: auto;
}
.recent-action {
  font-weight: 600;
  margin-right: 10px;
}
.recent-user {
  color: var(--el-text-color-secondary);
  margin-right: 10px;
  font-size: 12px;
}
.recent-detail {
  color: var(--el-text-color-regular);
  font-size: 12px;
}
</style>
