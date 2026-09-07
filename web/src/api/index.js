import http from './http'

export function login(data) {
  return http.post('/auth/login', data)
}

export function getMe() {
  return http.get('/auth/me')
}

export function logout() {
  return http.post('/auth/logout')
}

export function getDashboardSummary() {
  return http.get('/dashboard/summary')
}
