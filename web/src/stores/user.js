import { defineStore } from 'pinia'
import { getMe, login as loginApi } from '../api'
import { saveToken, clearToken } from '../api/http'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('domhub_token') || '',
    user: null,
  }),
  getters: {
    isLoggedIn: (state) => !!state.token,
  },
  actions: {
    async login(username, password) {
      const res = await loginApi({ username, password })
      // 开启两步验证的用户：仅返回中间态（require_2fa + pre_token），不写入 token
      if (res.data?.require_2fa) return res.data
      this.token = res.data.token
      this.user = res.data.user
      saveToken(this.token)
      return null
    },
    async fetchMe() {
      if (!this.token) return
      const res = await getMe()
      this.user = res.data
    },
    setToken(token) {
      this.token = token
      saveToken(token)
    },
    clear() {
      this.token = ''
      this.user = null
      clearToken()
    },
  },
})
