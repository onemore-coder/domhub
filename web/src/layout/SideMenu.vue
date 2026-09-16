<template>
  <div class="side-menu">
    <div class="logo">
      <div class="logo-mark">D</div>
      <div class="logo-text">
        <span class="logo-name">DomHub</span>
        <span class="logo-sub">多云域名管理</span>
      </div>
    </div>
    <el-menu
      :default-active="($route.meta.menu || $route.path)"
      router
      @select="emit('navigate')"
    >
      <el-menu-item index="/dashboard">
        <el-icon><Odometer /></el-icon>
        <span>仪表盘</span>
      </el-menu-item>
      <el-sub-menu index="domain-group">
        <template #title>
          <el-icon><Collection /></el-icon>
          <span>域名管理</span>
        </template>
        <el-menu-item index="/domains">域名列表</el-menu-item>
        <el-menu-item index="/dns">DNS 管理</el-menu-item>
      </el-sub-menu>
      <el-sub-menu index="cert-group">
        <template #title>
          <el-icon><Lock /></el-icon>
          <span>证书管理</span>
        </template>
        <el-menu-item index="/certs">证书监控</el-menu-item>
        <el-menu-item index="/certs/apply">证书申请</el-menu-item>
      </el-sub-menu>
      <el-menu-item index="/accounts">
        <el-icon><Cloudy /></el-icon>
        <span>云账号</span>
      </el-menu-item>
      <el-menu-item index="/alerts">
        <el-icon><Bell /></el-icon>
        <span>告警中心</span>
      </el-menu-item>
      <el-sub-menu index="security-group">
        <template #title>
          <el-icon><Key /></el-icon>
          <span>安全配置</span>
        </template>
        <el-menu-item v-if="isAdmin" index="/users">用户与权限</el-menu-item>
        <el-menu-item v-if="isAdmin" index="/audit">审计日志</el-menu-item>
        <el-menu-item index="/tokens">API Token</el-menu-item>
      </el-sub-menu>
      <el-menu-item index="/settings">
        <el-icon><Setting /></el-icon>
        <span>系统设置</span>
      </el-menu-item>
    </el-menu>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Collection, Bell, Cloudy, Key, Lock, Odometer, Setting } from '@element-plus/icons-vue'
import { useUserStore } from '../stores/user'

defineOptions({ name: 'SideMenu' })

const emit = defineEmits(['navigate'])
const route = useRoute()
const isAdmin = computed(() => useUserStore().user?.role === 'admin')
</script>

<style scoped>
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  flex-shrink: 0;
}
.logo-mark {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.logo-text {
  display: flex;
  flex-direction: column;
  line-height: 1.25;
}
.logo-name {
  color: var(--el-text-color-primary);
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.2px;
}
.logo-sub {
  color: var(--el-text-color-placeholder);
  font-size: 11px;
}
.el-menu {
  border-right: none;
  background: transparent;
  padding: 6px 10px;
  flex: 1;
  overflow-y: auto;
}
:deep(.el-menu-item),
:deep(.el-sub-menu__title) {
  height: 40px;
  line-height: 40px;
  border-radius: 8px;
  margin-bottom: 2px;
  color: var(--el-text-color-regular);
  /* 覆盖 EP 内联的 level padding，统一左缘：容器 10px + 项 8px = 图标起点 18px，与 Logo 对齐 */
  padding-left: 8px !important;
  padding-right: 10px;
  transition: background-color 0.15s ease, color 0.15s ease;
}
:deep(.el-menu-item:hover),
:deep(.el-sub-menu__title:hover) {
  background-color: var(--dh-hover);
  color: var(--el-text-color-primary);
}
:deep(.el-menu-item.is-active) {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 600;
}
:deep(.el-menu-item .el-icon),
:deep(.el-sub-menu__title .el-icon) {
  color: var(--el-text-color-secondary);
}
:deep(.el-menu-item.is-active .el-icon) {
  color: var(--el-color-primary);
}
/* 仅匹配展开分组内的二级项：18px(父级图标左缘) + 图标 18px + 间距 8px ≈ 文字对齐父级文字 */
:deep(.el-sub-menu .el-menu .el-menu-item) {
  padding-left: 34px !important;
  font-size: 13px;
  height: 36px;
  line-height: 36px;
  min-width: 0;
}
.side-menu {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--dh-sidebar-bg);
}
</style>
