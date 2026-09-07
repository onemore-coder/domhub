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

    <el-card shadow="never" class="welcome-card">
      <template #header>欢迎使用 DomHub</template>
      <el-empty description="M0 骨架阶段：云账号接入与域名台账将在 M1 上线" />
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getDashboardSummary } from '../api'

const loading = ref(true)
const summary = ref({})

onMounted(async () => {
  try {
    const res = await getDashboardSummary()
    summary.value = res.data || {}
  } finally {
    loading.value = false
  }
})

const cards = computed(() => [
  { label: '域名总数', value: summary.value.domain_total ?? 0, icon: 'Collection', color: '#409eff' },
  { label: '托管 Zone', value: summary.value.zone_total ?? 0, icon: 'Connection', color: '#67c23a' },
  { label: '云账号', value: summary.value.account_total ?? 0, icon: 'Cloudy', color: '#e6a23c' },
  { label: '30 天内到期', value: summary.value.expiring_in_30d ?? 0, icon: 'AlarmClock', color: '#f56c6c' },
])
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
  color: #1f2d3d;
}
.stat-label {
  font-size: 13px;
  color: #909399;
}
.welcome-card {
  margin-top: 8px;
}
</style>
