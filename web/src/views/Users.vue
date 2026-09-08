<template>
  <div class="users-page">
    <el-card shadow="never">
      <div class="toolbar">
        <span class="title">用户与权限</span>
        <div class="spacer" />
        <el-button type="primary" :icon="Plus" @click="openCreate">新增用户</el-button>
      </div>

      <el-table :data="users" v-loading="loading" stripe>
        <el-table-column label="ID" prop="id" width="60" />
        <el-table-column label="用户名" prop="username" min-width="120" />
        <el-table-column label="角色" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="roleTag(row.role)">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 1 ? 'success' : 'danger'">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近登录" width="170">
          <template #default="{ row }">
            {{ row.last_login_at ? new Date(row.last_login_at).toLocaleString('zh-CN', { hour12: false }) : '从未' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="row.id === me?.id" @click="openEdit(row)">编辑</el-button>
            <el-button
              link type="primary" :disabled="row.id === me?.id || row.role === 'admin'"
              @click="openGrants(row)"
            >Zone 授权</el-button>
            <el-button link type="danger" :disabled="row.id === me?.id" @click="removeUser(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑用户 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑用户' : '新增用户'" width="460px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="editing" placeholder="至少 2 位" />
        </el-form-item>
        <el-form-item :label="editing ? '重置密码' : '密码'">
          <el-input
            v-model="form.password" type="password" show-password
            :placeholder="editing ? '留空表示不修改' : '至少 6 位'"
          />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="管理员（全部权限）" value="admin" />
            <el-option label="操作员（DNS 操作，按授权）" value="operator" />
            <el-option label="观察者（只读）" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="editing" label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
        <el-alert
          v-if="form.role === 'operator' || form.role === 'viewer'" type="info" :closable="false"
          title="operator / viewer 只能访问被授权的 Zone（在列表中点击「Zone 授权」进行分配）"
        />
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">确定</el-button>
      </template>
    </el-dialog>

    <!-- Zone 授权 -->
    <el-dialog v-model="grantDialogVisible" :title="`Zone 授权 - ${grantUser?.username || ''}`" width="560px">
      <el-alert type="info" :closable="false" show-icon class="grant-alert"
        title="勾选该用户可访问的 账号+Zone 组合；未勾选的 Zone 对其不可见（DNS 管理页面）" />
      <div v-loading="grantsLoading" class="grant-list">
        <div v-for="(zones, accId) in zonesByAccount" :key="accId" class="grant-account">
          <div class="grant-account-name">{{ accountName(accId) }}</div>
          <el-checkbox-group v-model="grantSelection[accId]">
            <el-checkbox v-for="z in zones" :key="z" :value="z">{{ z }}</el-checkbox>
          </el-checkbox-group>
        </div>
        <el-empty v-if="!Object.keys(zonesByAccount).length" description="暂无可用 Zone，请先在账号页完成同步" :image-size="60" />
      </div>
      <template #footer>
        <el-button @click="grantDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingGrants" @click="saveGrants">保存授权</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useUserStore } from '../stores/user'
import {
  listUsers, createUser, updateUser, deleteUser,
  getUserZones, setUserZones,
  listAccounts, listDNSZones,
} from '../api/domhub'

const userStore = useUserStore()
const me = computed(() => userStore.user)

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const form = ref({})
const editingId = ref(0)

const grantDialogVisible = ref(false)
const grantsLoading = ref(false)
const savingGrants = ref(false)
const grantUser = ref(null)
const zonesByAccount = ref({}) // accountId -> [zone...]
const grantSelection = ref({}) // accountId -> [zone...]

const roleLabel = (r) => ({ admin: '管理员', operator: '操作员', viewer: '观察者' }[r] || r)
const roleTag = (r) => ({ admin: 'danger', operator: 'warning', viewer: 'info' }[r] || 'info')

function accountName(accId) {
  const a = accountsCache.value.find((x) => String(x.id) === String(accId))
  return a ? `${a.name}（${a.provider}）` : `账号 #${accId}`
}
const accountsCache = ref([])

async function load() {
  loading.value = true
  try {
    const res = await listUsers()
    users.value = res.data || []
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
  // 拉账号与其托管 Zone（用于授权对话框，从 DNS API 实时获取）
  try {
    const accRes = await listAccounts()
    accountsCache.value = accRes.data?.items || []
    const byAccount = {}
    for (const a of accountsCache.value) {
      try {
        const res = await listDNSZones(a.id)
        byAccount[String(a.id)] = (res.data || []).map((z) => z.name)
      } catch {
        byAccount[String(a.id)] = []
      }
      // 初始化勾选状态，保证 v-model 可写
      grantSelection.value[String(a.id)] = grantSelection.value[String(a.id)] || []
    }
    zonesByAccount.value = byAccount
  } catch { /* 授权对话框数据加载失败不阻断 */ }
})

function openCreate() {
  editing.value = false
  form.value = { username: '', password: '', role: 'viewer', enabled: true }
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = true
  editingId.value = row.id
  form.value = { username: row.username, password: '', role: row.role, enabled: row.status === 1 }
  dialogVisible.value = true
}

async function save() {
  const f = form.value
  if (!editing.value && (!f.username || !f.password)) {
    ElMessage.warning('用户名与密码不能为空')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateUser(editingId.value, {
        role: f.role,
        status: f.enabled ? 1 : 0,
        password: f.password || '',
      })
      ElMessage.success('已更新')
    } else {
      await createUser({ username: f.username, password: f.password, role: f.role })
      ElMessage.success('用户已创建')
    }
    dialogVisible.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function removeUser(row) {
  try {
    await ElMessageBox.confirm(`确认删除用户 ${row.username}？该用户的 Zone 授权将一并移除。`, '删除用户', {
      type: 'warning', confirmButtonText: '删除',
    })
  } catch { return }
  try {
    await deleteUser(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '删除失败')
  }
}

async function openGrants(row) {
  grantUser.value = row
  grantDialogVisible.value = true
  grantsLoading.value = true
  grantSelection.value = {}
  try {
    const res = await getUserZones(row.id)
    for (const g of res.data || []) {
      const key = String(g.cloud_account_id)
      grantSelection.value[key] = grantSelection.value[key] || []
      grantSelection.value[key].push(g.zone)
    }
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '拉取授权失败')
  } finally {
    grantsLoading.value = false
  }
}

async function saveGrants() {
  const grants = []
  for (const [accId, zones] of Object.entries(grantSelection.value)) {
    for (const z of zones || []) {
      grants.push({ account_id: Number(accId), zone: z })
    }
  }
  savingGrants.value = true
  try {
    await setUserZones(grantUser.value.id, grants)
    ElMessage.success('授权已保存')
    grantDialogVisible.value = false
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    savingGrants.value = false
  }
}
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
.title {
  font-weight: 600;
  color: #303133;
}
.spacer {
  flex: 1;
}
.grant-alert {
  margin-bottom: 14px;
}
.grant-list {
  max-height: 380px;
  overflow-y: auto;
}
.grant-account {
  margin-bottom: 14px;
}
.grant-account-name {
  font-weight: 600;
  margin-bottom: 6px;
  color: #303133;
}
</style>
