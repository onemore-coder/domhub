<template>
  <el-container class="main-layout">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <span class="logo-name">DomHub</span>
        <span class="logo-sub">多云域名管理</span>
      </div>
      <el-menu
        :default-active="$route.path"
        router
        background-color="#1f2d3d"
        text-color="#bfcbd9"
        active-text-color="#409eff"
      >
        <el-menu-item index="/dashboard">
          <el-icon><Odometer /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/accounts">
          <el-icon><Cloudy /></el-icon>
          <span>云账号</span>
        </el-menu-item>
        <el-menu-item index="/domains">
          <el-icon><Collection /></el-icon>
          <span>域名台账</span>
        </el-menu-item>
        <el-menu-item index="/alerts">
          <el-icon><Bell /></el-icon>
          <span>告警中心</span>
        </el-menu-item>
        <el-menu-item index="/dns">
          <el-icon><Connection /></el-icon>
          <span>DNS 管理</span>
        </el-menu-item>
        <el-menu-item index="/audit">
          <el-icon><Document /></el-icon>
          <span>审计日志</span>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <span>系统设置</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-title">{{ $route.meta.title }}</div>
        <el-dropdown @command="handleCommand">
          <span class="user-info">
            <el-avatar :size="30" class="user-avatar">{{ initial }}</el-avatar>
            {{ userStore.user?.username || '用户' }}
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>

      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()

const initial = computed(() => (userStore.user?.username || 'U').charAt(0).toUpperCase())

onMounted(() => {
  userStore.fetchMe()
})

function handleCommand(cmd) {
  if (cmd === 'logout') {
    userStore.clear()
    router.push('/login')
  }
}
</script>

<style scoped>
.main-layout {
  height: 100%;
}
.aside {
  background-color: #1f2d3d;
}
.logo {
  height: 60px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding-left: 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}
.logo-name {
  color: #fff;
  font-size: 20px;
  font-weight: 600;
  letter-spacing: 1px;
}
.logo-sub {
  color: #6b7a8d;
  font-size: 11px;
}
.aside .el-menu {
  border-right: none;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #e6e8eb;
}
.header-title {
  font-size: 16px;
  font-weight: 600;
  color: #1f2d3d;
}
.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: #303133;
  font-size: 14px;
}
.user-avatar {
  background: #409eff;
}
.main {
  background: #f5f7fa;
}
</style>
