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

// 告警
export const listChannels = () => http.get('/channels')
export const createChannel = (data) => http.post('/channels', data)
export const updateChannel = (id, data) => http.put(`/channels/${id}`, data)
export const deleteChannel = (id) => http.delete(`/channels/${id}`)
export const listRules = () => http.get('/alert-rules')
export const createRule = (data) => http.post('/alert-rules', data)
export const updateRule = (id, data) => http.put(`/alert-rules/${id}`, data)
export const deleteRule = (id) => http.delete(`/alert-rules/${id}`)
export const runAlertCheck = () => http.post('/alerts/check')
export const listAlertLogs = (limit = 50) => http.get('/alerts/logs', { params: { limit } })

// DNS 解析管理
export const listDNSZones = (accountId) => http.get('/dns/zones', { params: { account_id: accountId } })
export const listDNSRecords = (accountId, zone) => http.get('/dns/records', { params: { account_id: accountId, zone } })
export const createDNSRecord = (data) => http.post('/dns/records', data)
export const updateDNSRecord = (data) => http.put('/dns/records', data)
export const deleteDNSRecord = (data) => http.delete('/dns/records', { data })
export const planDNS = (data) => http.post('/dns/plan', data)
export const pushDNS = (data) => http.post('/dns/push', data)

// 审计日志
export const listAuditLogs = (params) => http.get('/audit-logs', { params })
