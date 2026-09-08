import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '../stores/user'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: { title: '登录' },
  },
  {
    path: '/',
    component: () => import('../layout/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: '仪表盘' },
      },
      // M1：云账号 / 域名台账 / 告警中心
      {
        path: 'accounts',
        name: 'Accounts',
        component: () => import('../views/Accounts.vue'),
        meta: { title: '云账号' },
      },
      {
        path: 'domains',
        name: 'Domains',
        component: () => import('../views/Domains.vue'),
        meta: { title: '域名台账' },
      },
      {
        path: 'dns',
        name: 'DNS',
        component: () => import('../views/Dns.vue'),
        meta: { title: 'DNS 管理' },
      },
      {
        path: 'alerts',
        name: 'Alerts',
        component: () => import('../views/Alerts.vue'),
        meta: { title: '告警中心' },
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('../views/Audit.vue'),
        meta: { title: '审计日志', adminOnly: true },
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('../views/Users.vue'),
        meta: { title: '用户与权限', adminOnly: true },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('../views/Placeholder.vue'),
        meta: { title: '系统设置' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 登录守卫
router.beforeEach(async (to) => {
  document.title = `${to.meta.title || ''} · DomHub`
  const store = useUserStore()
  if (to.path !== '/login' && !store.isLoggedIn) {
    return '/login'
  }
  if (to.path === '/login' && store.isLoggedIn) {
    return '/dashboard'
  }
  // admin 页面守卫（store.user 未加载时先拉取）
  if (to.meta.adminOnly && store.isLoggedIn && !store.user) {
    await store.fetchMe().catch(() => {})
  }
  if (to.meta.adminOnly && store.user?.role !== 'admin') {
    return '/dashboard'
  }
  return true
})

export default router
