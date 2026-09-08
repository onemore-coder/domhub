<template>
  <div class="oauth-callback">
    <el-spin v-if="false" />
    <div class="msg">{{ message }}</div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()
const message = ref('GitHub 登录中...')

onMounted(async () => {
  // token 由后端通过 URL fragment 带回（#/oauth/callback#token=xxx），避免进服务端日志
  const m = window.location.hash.match(/token=([^&]+)/)
  if (!m) {
    message.value = '登录失败：未收到令牌'
    setTimeout(() => router.replace('/login'), 1500)
    return
  }
  userStore.setToken(m[1])
  try {
    await userStore.fetchMe()
    message.value = '登录成功，正在跳转...'
    router.replace('/dashboard')
  } catch {
    message.value = '登录失败：令牌无效'
    setTimeout(() => router.replace('/login'), 1500)
  }
})
</script>

<style scoped>
.oauth-callback {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
}
</style>
