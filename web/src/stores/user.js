import { defineStore } from 'pinia'
import { getMe, login as loginApi } from '../api'

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
      this.token = res.data.token
      this.user = res.data.user
      localStorage.setItem('domhub_token', this.token)
    },
    async fetchMe() {
      if (!this.token) return
      const res = await getMe()
      this.user = res.data
    },
    setToken(token) {
      this.token = token
      localStorage.setItem('domhub_token', token)
    },
    clear() {
      this.token = ''
      this.user = null
      localStorage.removeItem('domhub_token')
    },
  },
})
