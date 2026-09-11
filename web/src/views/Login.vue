<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-title">
        <div class="login-mark">D</div>
        <h2>DomHub</h2>
        <p>多云域名与 DNS 统一管理平台</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" size="large" @keyup.enter="handleLogin">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="用户名" :prefix-icon="User" clearable />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            :prefix-icon="Lock"
            show-password
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" class="login-btn" :loading="loading" @click="handleLogin">
            登 录
          </el-button>
        </el-form-item>
      </el-form>
      <div v-if="githubEnabled" class="oauth-divider"><span>或</span></div>
      <el-button v-if="githubEnabled" class="login-btn github-btn" @click="githubLogin">
        <svg viewBox="0 0 16 16" width="18" height="18" fill="currentColor" aria-hidden="true" class="github-icon">
          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
        </svg>
        使用 GitHub 登录
      </el-button>
      <div class="login-tip">初始账号 admin / admin123，登录后请及时修改</div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { useUserStore } from '../stores/user'
import http from '../api/http'

const router = useRouter()
const userStore = useUserStore()

const formRef = ref()
const loading = ref(false)
const githubEnabled = ref(false)
const form = reactive({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

onMounted(async () => {
  // OAuth 报错提示（回调重定向带回）
  const err = new URLSearchParams(window.location.search).get('oauth_error')
  if (err) {
    ElMessage.error(err === 'invalid' ? 'OAuth 授权参数无效' : `GitHub 登录失败: ${err}`)
  }
  // 是否启用 GitHub 登录
  try {
    const res = await http.get('/auth/oauth/providers')
    githubEnabled.value = !!res.data?.github
  } catch { /* 未启用不显示按钮 */ }
})

function githubLogin() {
  window.location.href = '/api/v1/auth/oauth/github'
}

async function handleLogin() {
  await formRef.value.validate()
  loading.value = true
  try {
    await userStore.login(form.username, form.password)
    ElMessage.success('登录成功')
    router.push('/dashboard')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--dh-canvas);
  background-image:
    radial-gradient(ellipse 80% 60% at 20% 0%, rgba(79, 70, 229, 0.12), transparent),
    radial-gradient(ellipse 60% 50% at 85% 90%, rgba(124, 58, 237, 0.1), transparent);
}
.login-card {
  width: 380px;
  padding: 44px 36px 24px;
  background: var(--dh-card-bg);
  backdrop-filter: blur(12px);
  border: 1px solid var(--dh-card-border);
  border-radius: 16px;
  box-shadow: 0 16px 48px rgba(24, 24, 27, 0.08);
}
.login-title {
  text-align: center;
  margin-bottom: 28px;
}
.login-mark {
  width: 48px;
  height: 48px;
  margin: 0 auto 14px;
  border-radius: 13px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  font-size: 24px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
.login-title h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  letter-spacing: 0.5px;
}
.login-title p {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.login-btn {
  width: 100%;
  font-weight: 600;
}
.login-tip {
  text-align: center;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
.oauth-divider {
  display: flex;
  align-items: center;
  margin: 4px 0 14px;
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
.oauth-divider::before,
.oauth-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--el-border-color);
}
.oauth-divider span {
  padding: 0 12px;
}
.github-btn {
  color: var(--el-text-color-primary);
  margin-bottom: 16px;
  font-weight: 500;
}
.github-icon {
  margin-right: 8px;
}
</style>
