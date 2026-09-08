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
        <el-menu-item v-if="isAdmin" index="/audit">
          <el-icon><Document /></el-icon>
          <span>审计日志</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/users">
          <el-icon><User /></el-icon>
          <span>用户与权限</span>
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
import { useUserStore } from '../stores/user'
import { changeMyPassword } from '../api/domhub'

const router = useRouter()
const userStore = useUserStore()

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
