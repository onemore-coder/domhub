import http from './http'

// 云账号
export const listAccounts = () => http.get('/accounts')
export const createAccount = (data) => http.post('/accounts', data)
export const updateAccount = (id, data) => http.put(`/accounts/${id}`, data)
export const deleteAccount = (id) => http.delete(`/accounts/${id}`)
export const checkAccount = (id) => http.post(`/accounts/${id}/check`)
export const syncAccount = (id) => http.post(`/accounts/${id}/sync`)

// 域名台账
export const listDomains = (params) => http.get('/domains', { params })
export const syncAllDomains = () => http.post('/domains/sync')
export const updateDomainMeta = (id, data) => http.patch(`/domains/${id}`, data)

// 告警
export const runCertCheck = (data = {}) => http.post('/certs/check', data)
export const checkCertByID = (id) => http.post(`/certs/${id}/check`)
export const listCerts = () => http.get('/certs')
export const addCertHost = (data) => http.post('/certs', data)
export const deleteCert = (id) => http.delete(`/certs/${id}`)
export const setCertExcluded = (id, excluded) => http.put(`/certs/${id}/excluded`, { excluded })
export const listChannels = () => http.get('/channels')
export const createChannel = (data) => http.post('/channels', data)
export const updateChannel = (id, data) => http.put(`/channels/${id}`, data)
export const deleteChannel = (id) => http.delete(`/channels/${id}`)
export const testChannel = (data) => http.post('/channels/test', data)
export const testChannelByID = (id) => http.post(`/channels/${id}/test`)
export const listRules = () => http.get('/alert-rules')
export const createRule = (data) => http.post('/alert-rules', data)
export const updateRule = (id, data) => http.put(`/alert-rules/${id}`, data)
export const deleteRule = (id) => http.delete(`/alert-rules/${id}`)
export const runAlertCheck = () => http.post('/alerts/check')
export const listAlertLogs = (limit = 50) => http.get('/alerts/logs', { params: { limit } })

// DNS 解析管理
export const listDNSZones = () => http.get('/dns/zones') // 走本地缓存，秒开
export const refreshDNSZones = (accountId = 0) => http.post('/dns/zones/refresh', { account_id: accountId })
export const listDNSRecords = (accountId, zone) => http.get('/dns/records', { params: { account_id: accountId, zone } })
export const listDNSRecordsCached = (accountId, zone) => http.get('/dns/records-cached', { params: { account_id: accountId, zone } })
export const syncDNSRecords = (accountId, zone) => http.post('/dns/records/sync', { account_id: accountId, zone })
export const createDNSRecord = (data) => http.post('/dns/records', data)
export const updateDNSRecord = (data) => http.put('/dns/records', data)
export const deleteDNSRecord = (data) => http.delete('/dns/records', { data })
export const planDNS = (data) => http.post('/dns/plan', data)
export const pushDNS = (data) => http.post('/dns/push', data)

// 解析记录快照
export const listSnapshots = (accountId, zone, limit = 50) => http.get('/dns/snapshots', { params: { account_id: accountId, zone, limit } })
export const captureSnapshot = (data) => http.post('/dns/snapshots', data)
export const getSnapshot = (id) => http.get(`/dns/snapshots/${id}`)
export const diffSnapshots = (baseId, targetId) => http.post('/dns/snapshots/diff', { base_id: baseId, target_id: targetId })
export const restorePlan = (data) => http.post('/dns/snapshots/restore-plan', data)

// 系统设置
export const getSettings = () => http.get('/settings')
export const updateSchedules = (specs) => http.put('/settings/schedules', specs)

// API Token（个人管理）
export const listTokens = () => http.get('/tokens')
export const createToken = (data) => http.post('/tokens', data)
export const revokeToken = (id) => http.delete(`/tokens/${id}`)

// 审计日志
export const listAuditLogs = (params) => http.get('/audit-logs', { params })

// 用户管理
export const listUsers = () => http.get('/users')
export const createUser = (data) => http.post('/users', data)
export const updateUser = (id, data) => http.put(`/users/${id}`, data)
export const deleteUser = (id) => http.delete(`/users/${id}`)
export const getUserZones = (id) => http.get(`/users/${id}/zones`)
export const setUserZones = (id, grants) => http.put(`/users/${id}/zones`, { grants })
export const changeMyPassword = (data) => http.post('/users/me/password', data)
