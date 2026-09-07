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
      // M1+ 占位：域名台账 / DNS 管理 / 审计 / 设置
      {
        path: 'domains',
        name: 'Domains',
        component: () => import('../views/Placeholder.vue'),
        meta: { title: '域名台账' },
      },
      {
        path: 'dns',
        name: 'DNS',
        component: () => import('../views/Placeholder.vue'),
        meta: { title: 'DNS 管理' },
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('../views/Placeholder.vue'),
        meta: { title: '审计日志' },
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
router.beforeEach((to) => {
  document.title = `${to.meta.title || ''} · DomHub`
  const store = useUserStore()
  if (to.path !== '/login' && !store.isLoggedIn) {
    return '/login'
  }
  if (to.path === '/login' && store.isLoggedIn) {
    return '/dashboard'
  }
  return true
})

export default router
