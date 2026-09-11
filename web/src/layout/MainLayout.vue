<template>
  <el-container class="main-layout">
    <el-aside width="220px" class="aside">
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
          <el-menu-item index="/domains">域名台账</el-menu-item>
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
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-title">{{ $route.meta.title }}</div>
        <div class="header-search">
          <el-input
            v-model="searchKeyword" placeholder="搜索域名 / 解析记录，回车确认" clearable
            :prefix-icon="SearchIcon" @keyup.enter="doSearch"
          />
        </div>
        <el-dropdown @command="handleCommand">
          <span class="user-info">
            <el-avatar :size="30" class="user-avatar">{{ initial }}</el-avatar>
            {{ userStore.user?.username || '用户' }}
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="password">修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>

      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>

    <!-- 全局搜索结果 -->
    <el-dialog v-model="searchVisible" title="全局搜索" width="620px">
      <div v-loading="searching">
        <template v-if="searchKeyword">
          <div class="search-section">托管域名</div>
          <template v-if="searchZones.length">
            <div v-for="z in searchZones" :key="z.id" class="search-item" @click="goZoneRecord(z.cloud_account_id, z.name)">
              <el-icon class="search-item-icon"><Collection /></el-icon>
              <span class="search-item-main">{{ z.name }}</span>
              <span class="search-item-sub">{{ z.account_name }} · {{ z.record_count }} 条记录</span>
            </div>
          </template>
          <div v-else class="search-empty">无匹配域名</div>

          <div class="search-section">解析记录</div>
          <template v-if="searchRecords.length">
            <div
              v-for="r in searchRecords" :key="r.id"
              class="search-item" @click="goZoneRecord(r.cloud_account_id, r.zone_name, r.name)"
            >
              <el-tag size="small" :type="r.type === 'A' ? 'success' : 'info'" class="search-item-type">{{ r.type }}</el-tag>
              <span class="search-item-main">{{ r.name }}.{{ r.zone_name }}</span>
              <span class="search-item-value">{{ r.value }}</span>
              <span class="search-item-sub">{{ r.account_name }}</span>
            </div>
          </template>
          <div v-else class="search-empty">无匹配记录</div>
        </template>
      </div>
    </el-dialog>

    <!-- 修改密码 -->
    <el-dialog v-model="pwdDialogVisible" title="修改密码" width="420px">
      <el-form :model="pwdForm" label-width="90px">
        <el-form-item label="旧密码">
          <el-input v-model="pwdForm.old_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwdForm.new_password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="pwdForm.confirm" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="changingPwd" @click="doChangePassword">确定</el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Search as SearchIcon } from '@element-plus/icons-vue'
import { useUserStore } from '../stores/user'
import { changeMyPassword, globalSearch } from '../api/domhub'

const router = useRouter()
const userStore = useUserStore()

// ---- 全局搜索 ----
const searchKeyword = ref('')
const searchVisible = ref(false)
const searching = ref(false)
const searchZones = ref([])
const searchRecords = ref([])

async function doSearch() {
  const q = searchKeyword.value.trim()
  if (!q) return
  searchVisible.value = true
  searching.value = true
  try {
    const res = await globalSearch(q)
    searchZones.value = res.data?.zones || []
    searchRecords.value = res.data?.records || []
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '搜索失败')
  } finally {
    searching.value = false
  }
}

function goZoneRecord(accountId, zone, recordName) {
  searchVisible.value = false
  router.push({
    path: '/dns/records',
    query: { account_id: accountId, zone, ...(recordName ? { q: recordName } : {}) },
  })
}

const initial = computed(() => (userStore.user?.username || 'U').charAt(0).toUpperCase())
const isAdmin = computed(() => userStore.user?.role === 'admin')

onMounted(() => {
  userStore.fetchMe()
})

const pwdDialogVisible = ref(false)
const changingPwd = ref(false)
const pwdForm = reactive({ old_password: '', new_password: '', confirm: '' })

function handleCommand(cmd) {
  if (cmd === 'logout') {
    userStore.clear()
    router.push('/login')
  } else if (cmd === 'password') {
    pwdForm.old_password = ''
    pwdForm.new_password = ''
    pwdForm.confirm = ''
    pwdDialogVisible.value = true
  }
}

async function doChangePassword() {
  if (!pwdForm.old_password || pwdForm.new_password.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (pwdForm.new_password !== pwdForm.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  changingPwd.value = true
  try {
    await changeMyPassword({
      old_password: pwdForm.old_password,
      new_password: pwdForm.new_password,
    })
    ElMessage.success('密码已修改')
    pwdDialogVisible.value = false
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '修改失败')
  } finally {
    changingPwd.value = false
  }
}
</script>

<style scoped>
.main-layout {
  height: 100%;
}

/* ---------- 侧边栏：浅色画布 ---------- */
.aside {
  background-color: var(--dh-sidebar-bg);
  border-right: 1px solid var(--dh-sidebar-border);
  display: flex;
  flex-direction: column;
}
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
.aside .el-menu {
  border-right: none;
  background: transparent;
  padding: 6px 10px;
  flex: 1;
  overflow-y: auto;
}
.aside :deep(.el-menu-item),
.aside :deep(.el-sub-menu__title) {
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
.aside :deep(.el-menu-item:hover),
.aside :deep(.el-sub-menu__title:hover) {
  background-color: #f0f0f2;
  color: var(--el-text-color-primary);
}
.aside :deep(.el-menu-item.is-active) {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 600;
}
.aside :deep(.el-menu-item .el-icon),
.aside :deep(.el-sub-menu__title .el-icon) {
  color: var(--el-text-color-secondary);
}
.aside :deep(.el-menu-item.is-active .el-icon) {
  color: var(--el-color-primary);
}
/* 仅匹配展开分组内的二级项：18px(父级图标左缘) + 图标 18px + 间距 8px ≈ 文字对齐父级文字 */
.aside :deep(.el-sub-menu .el-menu .el-menu-item) {
  padding-left: 34px !important;
  font-size: 13px;
  height: 36px;
  line-height: 36px;
  min-width: 0;
}

/* ---------- 顶栏 ---------- */
.header {
  display: flex;
  align-items: center;
  height: 56px;
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--el-border-color-light);
  position: sticky;
  top: 0;
  z-index: 10;
}
.header-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  min-width: 96px;
}
.header-search {
  flex: 1;
  max-width: 420px;
  margin: 0 auto;
}
.header-search :deep(.el-input__wrapper) {
  border-radius: 999px;
  background: #f4f4f5;
  box-shadow: 0 0 0 1px transparent inset;
  transition: background 0.15s ease, box-shadow 0.15s ease;
}
.header-search :deep(.el-input__wrapper:hover) {
  background: #efeff1;
}
.header-search :deep(.el-input__wrapper.is-focus) {
  background: #fff;
  box-shadow: 0 0 0 1.5px var(--el-color-primary-light-5) inset;
}
.header-search :deep(.el-input__inner) {
  height: 34px;
}
.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: var(--el-text-color-primary);
  font-size: 13px;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 8px;
  transition: background-color 0.15s ease;
}
.user-info:hover {
  background: #f0f0f2;
}
.user-avatar {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  font-weight: 600;
}
.header :deep(.el-dropdown) {
  margin-left: 12px;
}

/* ---------- 全局搜索结果 ---------- */
.search-section {
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-placeholder);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin: 14px 0 6px;
}
.search-section:first-child {
  margin-top: 0;
}
.search-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: 8px;
  cursor: pointer;
  min-width: 0;
  transition: background-color 0.12s ease;
}
.search-item:hover {
  background: #f4f4f5;
}
.search-item-icon {
  color: var(--el-color-primary);
}
.search-item-type {
  flex-shrink: 0;
}
.search-item-main {
  font-weight: 500;
  white-space: nowrap;
}
.search-item-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
}
.search-item-sub {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
.search-empty {
  color: var(--el-text-color-placeholder);
  font-size: 13px;
  padding: 4px 10px 10px;
}

/* ---------- 主区域 ---------- */
.main {
  background: var(--dh-canvas);
  overflow-y: auto;
}
</style>
