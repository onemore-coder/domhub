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
              <el-dropdown-item command="twofa">
                两步验证
                <el-tag v-if="userStore.user?.totp_enabled" size="small" type="success" class="twofa-tag">已开启</el-tag>
              </el-dropdown-item>
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
    <!-- 两步验证 -->
    <el-dialog v-model="twofaVisible" title="两步验证（TOTP）" width="min(460px, 94vw)">
      <template v-if="!twofaEnabled">
        <el-steps :active="twofaStep" align-center finish-status="success" simple>
          <el-step title="扫码" />
          <el-step title="验证开启" />
        </el-steps>
        <div v-if="twofaStep === 0" v-loading="twofaLoading" class="twofa-setup">
          <img v-if="twofaQR" :src="twofaQR" alt="TOTP 二维码" class="twofa-qr" />
          <div class="twofa-secret">
            <div class="twofa-secret-label">无法扫码？手动输入密钥：</div>
            <code class="twofa-secret-code">{{ twofaSecret }}</code>
          </div>
          <div class="twofa-hint">使用 Google Authenticator / 1Password / 腾讯身份验证器扫描，然后点击下一步</div>
        </div>
        <div v-else class="twofa-setup">
          <el-input
            v-model="twofaCode" placeholder="输入 App 中的 6 位动态码" maxlength="6" size="large"
            class="twofa-code-input" @keyup.enter="doEnable2FA"
          />
        </div>
      </template>
      <template v-else>
        <div class="twofa-hint" style="margin-bottom: 12px">
          两步验证已开启。关闭前请输入验证器中的动态码确认身份。
        </div>
        <el-input
          v-model="twofaCode" placeholder="6 位动态码" maxlength="6" size="large"
          @keyup.enter="doDisable2FA"
        />
      </template>
      <template #footer>
        <el-button @click="twofaVisible = false">取消</el-button>
        <el-button v-if="!twofaEnabled" type="primary" :loading="twofaLoading" @click="twofaStep === 0 ? nextSetupStep() : doEnable2FA()">
          {{ twofaStep === 0 ? '下一步' : '确认开启' }}
        </el-button>
        <el-button v-else type="danger" :loading="twofaLoading" @click="doDisable2FA">确认关闭</el-button>
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
import { changeMyPassword, disable2FA, enable2FA, globalSearch, setup2FA } from '../api/domhub'

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
  } else if (cmd === 'twofa') {
    openTwofa()
  }
}

// ---- 两步验证 ----
const twofaVisible = ref(false)
const twofaLoading = ref(false)
const twofaStep = ref(0) // 0 扫码 1 输码
const twofaQR = ref('')
const twofaSecret = ref('')
const twofaCode = ref('')
const twofaEnabled = computed(() => !!userStore.user?.totp_enabled)

async function openTwofa() {
  twofaCode.value = ''
  twofaStep.value = 0
  if (twofaEnabled.value) {
    twofaVisible.value = true
    return
  }
  twofaVisible.value = true
  twofaLoading.value = true
  try {
    const res = await setup2FA()
    twofaQR.value = res.data.qr
    twofaSecret.value = res.data.secret
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '生成密钥失败')
    twofaVisible.value = false
  } finally {
    twofaLoading.value = false
  }
}

function nextSetupStep() {
  if (!twofaSecret.value) return
  twofaStep.value = 1
}

async function doEnable2FA() {
  if (twofaCode.value.length !== 6) {
    ElMessage.warning('请输入 6 位动态码')
    return
  }
  twofaLoading.value = true
  try {
    await enable2FA(twofaCode.value)
    ElMessage.success('两步验证已开启，下次登录需要输入动态码')
    twofaVisible.value = false
    await userStore.fetchMe()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '验证失败')
  } finally {
    twofaLoading.value = false
  }
}

async function doDisable2FA() {
  if (twofaCode.value.length !== 6) {
    ElMessage.warning('请输入 6 位动态码')
    return
  }
  twofaLoading.value = true
  try {
    await disable2FA(twofaCode.value)
    ElMessage.success('两步验证已关闭')
    twofaVisible.value = false
    await userStore.fetchMe()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '验证失败')
  } finally {
    twofaLoading.value = false
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

/* ---------- 两步验证 ---------- */
.twofa-tag {
  margin-left: 6px;
}
.twofa-setup {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 0 4px;
}
.twofa-qr {
  width: 220px;
  height: 220px;
  border-radius: 8px;
  background: #fff;
  padding: 8px;
}
.twofa-secret {
  text-align: center;
}
.twofa-secret-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
.twofa-secret-code {
  display: inline-block;
  padding: 6px 12px;
  border-radius: 6px;
  background: var(--el-fill-color-light);
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 13px;
  letter-spacing: 1px;
  word-break: break-all;
}
.twofa-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  text-align: center;
}
.twofa-code-input {
  max-width: 260px;
}
.twofa-code-input :deep(input) {
  letter-spacing: 6px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
}
</style>
