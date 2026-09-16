<template>
  <el-container class="main-layout">
    <el-aside v-if="!isMobile" width="220px" class="aside">
      <SideMenu />
    </el-aside>

    <!-- 移动端侧边栏抽屉 -->
    <el-drawer
      v-model="sidebarOpen" direction="ltr" :with-header="false" size="248px"
      class="mobile-sidebar"
    >
      <SideMenu @navigate="sidebarOpen = false" />
    </el-drawer>

    <el-container>
      <el-header class="header">
        <el-button
          v-if="isMobile" class="menu-btn" text circle
          :icon="Fold" @click="sidebarOpen = true"
        />
        <div class="header-title">{{ $route.meta.title }}</div>
        <div v-if="!isMobile" class="header-search">
          <el-input
            v-model="searchKeyword" placeholder="搜索域名 / 解析记录，回车确认" clearable
            :prefix-icon="SearchIcon" @keyup.enter="doSearch"
          />
        </div>
        <el-tooltip :content="isDark ? '切换到浅色模式' : '切换到深色模式'" placement="bottom">
          <el-button class="theme-toggle" text circle @click="toggleTheme">
            <el-icon :size="17"><Sunny v-if="isDark" /><Moon v-else /></el-icon>
          </el-button>
        </el-tooltip>
        <el-dropdown @command="handleCommand">
          <span class="user-info">
            <el-avatar :size="30" class="user-avatar">{{ initial }}</el-avatar>
            <span v-if="!isMobile" class="user-name">{{ userStore.user?.username || '用户' }}</span>
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
    <el-dialog v-model="searchVisible" title="全局搜索" width="min(620px, 94vw)">
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
    <el-dialog v-model="pwdDialogVisible" title="修改密码" width="min(420px, 94vw)">
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
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Fold, Search as SearchIcon } from '@element-plus/icons-vue'
import SideMenu from './SideMenu.vue'
import { useUserStore } from '../stores/user'
import { changeMyPassword, globalSearch } from '../api/domhub'

const router = useRouter()
const userStore = useUserStore()

// ---- 移动端适配：≤768px 侧边栏收进抽屉、隐藏顶栏搜索 ----
const mq = window.matchMedia('(max-width: 768px)')
const isMobile = ref(mq.matches)
const sidebarOpen = ref(false)
const onMqChange = (e) => {
  isMobile.value = e.matches
  if (!e.matches) sidebarOpen.value = false
}
onMounted(() => mq.addEventListener('change', onMqChange))
onBeforeUnmount(() => mq.removeEventListener('change', onMqChange))

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

// ---- 深浅色模式 ----
const isDark = ref(localStorage.getItem('domhub-theme') === 'dark')
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('domhub-theme', isDark.value ? 'dark' : 'light')
}

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

/* ---------- 侧边栏：浅色画布（菜单本体在 SideMenu.vue） ---------- */
.aside {
  background-color: var(--dh-sidebar-bg);
  border-right: 1px solid var(--dh-sidebar-border);
}

/* ---------- 顶栏 ---------- */
.header {
  display: flex;
  align-items: center;
  height: 56px;
  background: var(--dh-header-bg);
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
  background: var(--dh-input-bg);
  box-shadow: 0 0 0 1px transparent inset;
  transition: background 0.15s ease, box-shadow 0.15s ease;
}
.header-search :deep(.el-input__wrapper:hover) {
  background: var(--dh-hover);
}
.header-search :deep(.el-input__wrapper.is-focus) {
  background: var(--el-bg-color);
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
  background: var(--dh-hover);
}
.user-avatar {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  font-weight: 600;
}
.menu-btn {
  margin-right: 6px;
  color: var(--el-text-color-regular);
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
  background: var(--dh-input-bg);
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
