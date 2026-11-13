<template>
  <div class="connman">
    <div class="cm-head">
      <h2>连接管理</h2>
      <span class="grow"></span>
      <button class="btn primary" @click="startNew"><Icon name="plus" :size="15" /> 新建连接</button>
    </div>

    <!-- 连接列表 -->
    <div class="cm-list" v-if="!showForm">
      <div v-for="g in grouped" :key="g.name" class="cm-group">
        <div class="cm-group-label">
          <Icon name="folder" :size="13" />
          {{ g.name }}
          <span class="faint">（{{ g.items.length }}）</span>
        </div>
        <div v-for="c in g.items" :key="c.id" class="cm-item card" :class="{ active: c.id === store.currentConnId }">
          <span class="dot" :class="c.health" :title="c.healthError"></span>
          <div class="cm-info" @dblclick="openConn(c)">
            <div class="cm-name">
              {{ c.name }}
              <span v-if="c.readOnly" class="tag ro-tag"><Icon name="lock" :size="10" /> 只读</span>
              <span v-if="c.mode !== 'standalone'" class="tag mode-tag">{{ c.mode }}</span>
              <span v-if="c.ssh.enabled" class="tag ssh-tag">SSH</span>
              <span v-if="c.tls.enabled" class="tag tls-tag">TLS</span>
            </div>
            <div class="cm-addr faint mono">{{ c.host }}:{{ c.port }} · DB{{ c.mode === 'cluster' ? 0 : c.db }} · {{ c.health === 'ok' ? '已连接' : c.health === 'down' ? '断线' : c.health === 'connecting' ? '连接中' : '未连接' }}</div>
          </div>
          <span class="grow"></span>
          <button v-if="c.id !== store.currentConnId" class="btn sm primary" @click="openConn(c)">连接</button>
          <button v-else class="btn sm danger-ghost" @click="closeConn(c)">断开</button>
          <button class="btn sm ghost" @click="editConn(c)" title="编辑"><Icon name="edit" :size="14" /></button>
          <button class="btn sm ghost" @click="askDelete(c)" title="删除"><Icon name="trash" :size="14" style="color: var(--red)" /></button>
        </div>
      </div>

      <div v-if="!store.connections.length" class="empty" style="flex: 1">
        <img :src="logoPng" style="width: 84px; opacity: 0.9" alt="RedisLens" />
        <h3 style="margin: 0">欢迎使用 RedisLens</h3>
        <div class="faint">创建第一个连接，开始管理你的 Redis 实例</div>
        <button class="btn primary" @click="startNew"><Icon name="plus" :size="15" /> 新建连接</button>
        <div class="faint" style="font-size: 12px; margin-top: 8px">
          从创建连接到完成首次数据浏览 ≤ 1 分钟
        </div>
      </div>
    </div>

    <!-- 连接表单 -->
    <div v-else class="cm-form card">
      <div class="form-title">
        <h3>{{ editing ? '编辑连接' : '新建连接' }}</h3>
        <span class="grow"></span>
        <button class="btn ghost sm" @click="showForm = false"><Icon name="x" :size="15" /></button>
      </div>

      <div class="form-grid">
        <div class="f-field">
          <label>名称 *</label>
          <input v-model="form.name" placeholder="如 生产-订单缓存" />
        </div>
        <div class="f-field">
          <label>分组（环境）</label>
          <input v-model="form.group" list="group-list" placeholder="开发 / 测试 / 生产" />
          <datalist id="group-list">
            <option v-for="g in groupNames" :key="g" :value="g" />
          </datalist>
        </div>
        <div class="f-field">
          <label>模式</label>
          <select v-model="form.mode">
            <option value="standalone">单机</option>
            <option value="sentinel">哨兵（Sentinel）</option>
            <option value="cluster">集群（Cluster）</option>
          </select>
        </div>
        <div class="f-field">
          <label>地址</label>
          <input v-model="form.host" class="mono" placeholder="127.0.0.1" />
        </div>
        <div class="f-field">
          <label>端口</label>
          <input v-model.number="form.port" type="number" class="mono" placeholder="6379" />
        </div>
        <div class="f-field">
          <label>数据库（DB）</label>
          <input v-model.number="form.db" type="number" min="0" max="15" class="mono" :disabled="form.mode === 'cluster'" />
        </div>
        <div class="f-field">
          <label>用户名（ACL，可选）</label>
          <input v-model="form.username" class="mono" placeholder="default" />
        </div>
        <div class="f-field">
          <label>密码 <span v-if="editing && formHasPassword" class="faint" style="font-weight: 400">（留空沿用原密码）</span></label>
          <input v-model="form.password" type="password" class="mono" placeholder="••••••••" />
        </div>
      </div>

      <!-- 哨兵配置 -->
      <div v-if="form.mode === 'sentinel'" class="form-section">
        <h4>哨兵配置</h4>
        <div class="form-grid">
          <div class="f-field">
            <label>主库名称（master name）</label>
            <input v-model="form.sentinel.master" class="mono" placeholder="mymaster" />
          </div>
          <div class="f-field">
            <label>哨兵地址（逗号分隔）</label>
            <input v-model="sentinelAddrsText" class="mono" placeholder="10.0.0.1:26379, 10.0.0.2:26379" />
          </div>
          <div class="f-field">
            <label>哨兵用户名（可选）</label>
            <input v-model="form.sentinel.username" class="mono" />
          </div>
          <div class="f-field">
            <label>哨兵密码（可选）</label>
            <input v-model="form.sentinel.password" type="password" class="mono" />
          </div>
        </div>
      </div>

      <!-- SSH 隧道 -->
      <div class="form-section">
        <h4 class="toggle-head" @click="form.ssh.enabled = !form.ssh.enabled">
          <span class="toggle" :class="{ on: form.ssh.enabled }"></span>
          SSH 隧道
        </h4>
        <div v-if="form.ssh.enabled" class="form-grid">
          <div class="f-field">
            <label>SSH 主机</label>
            <input v-model="form.ssh.host" class="mono" />
          </div>
          <div class="f-field">
            <label>SSH 端口</label>
            <input v-model.number="form.ssh.port" type="number" class="mono" placeholder="22" />
          </div>
          <div class="f-field">
            <label>用户名</label>
            <input v-model="form.ssh.user" class="mono" />
          </div>
          <div class="f-field">
            <label>认证方式</label>
            <select v-model="form.ssh.authType">
              <option value="password">密码</option>
              <option value="key">私钥</option>
            </select>
          </div>
          <div class="f-field" v-if="form.ssh.authType === 'password'">
            <label>SSH 密码 <span v-if="editing && formHasSSHPass" class="faint" style="font-weight: 400">（留空沿用）</span></label>
            <input v-model="form.ssh.password" type="password" class="mono" />
          </div>
          <div class="f-field" v-else>
            <label>私钥路径</label>
            <div class="btn-row">
              <input v-model="form.ssh.keyPath" class="mono grow" placeholder="C:\Users\you\.ssh\id_rsa" />
              <button class="btn sm" @click="pickKey">浏览…</button>
            </div>
          </div>
          <div class="f-field" v-if="form.ssh.authType === 'key'">
            <label>私钥口令（可选）</label>
            <input v-model="form.ssh.passphrase" type="password" class="mono" />
          </div>
        </div>
      </div>

      <!-- TLS -->
      <div class="form-section">
        <h4 class="toggle-head" @click="form.tls.enabled = !form.tls.enabled">
          <span class="toggle" :class="{ on: form.tls.enabled }"></span>
          TLS 加密
        </h4>
        <div v-if="form.tls.enabled" class="form-grid">
          <div class="f-field">
            <label>CA 证书（可选）</label>
            <div class="btn-row">
              <input v-model="form.tls.caFile" class="mono grow" />
              <button class="btn sm" @click="pickCA">浏览…</button>
            </div>
          </div>
          <div class="f-field">
            <label>客户端证书（可选）</label>
            <div class="btn-row">
              <input v-model="form.tls.certFile" class="mono grow" />
              <button class="btn sm" @click="pickCert">浏览…</button>
            </div>
          </div>
          <div class="f-field">
            <label>客户端私钥（可选）</label>
            <div class="btn-row">
              <input v-model="form.tls.keyFile" class="mono grow" />
              <button class="btn sm" @click="pickTLSKey">浏览…</button>
            </div>
          </div>
          <div class="f-field">
            <label>SNI 服务器名（可选）</label>
            <input v-model="form.tls.serverName" class="mono" />
          </div>
          <div class="f-field">
            <label class="chk-row"><input type="checkbox" v-model="form.tls.insecureSkipVerify" /> 跳过证书校验（自签证书）</label>
          </div>
        </div>
      </div>

      <!-- 只读 -->
      <div class="form-section">
        <h4 class="toggle-head" @click="form.readOnly = !form.readOnly">
          <span class="toggle" :class="{ on: form.readOnly }"></span>
          只读模式（生产环境推荐，所有写操作入口隐藏）
        </h4>
      </div>

      <div class="form-actions">
        <button class="btn" @click="showForm = false">取消</button>
        <button class="btn" :disabled="testing" @click="test">
          <span v-if="testing" class="spinner"></span>
          <Icon v-else name="plug" :size="14" />
          {{ testing ? '测试中…' : '测试连接' }}
        </button>
        <button class="btn primary" @click="save" :disabled="!form.name">保存</button>
      </div>
      <div v-if="testState.done" class="test-result card fade-in">
        <template v-if="testState.ok">
          <b style="color: var(--green)">✓ 连接成功</b> — Redis {{ testState.summary!.version }} · {{ testState.summary!.role }} ·
          {{ testState.summary!.mode }} · 内存 {{ fmtBytes(testState.summary!.usedMem) }} ·
          {{ testState.summary!.totalKeys >= 0 ? testState.summary!.totalKeys.toLocaleString() + ' 键' : '-' }}
        </template>
        <template v-else><b style="color: var(--red)">✗ 连接失败</b> — {{ testState.message }}</template>
      </div>
    </div>

    <!-- 删除连接确认（根级：列表视图下也可弹出） -->
    <ConfirmDialog
      :visible="!!delTarget"
      title="删除连接"
      :danger="true"
      confirm-text="删除"
      @confirm="doDelete"
      @cancel="delTarget = null"
    >
      将删除连接「{{ delTarget?.name }}」及其保存的凭据（不影响 Redis 服务端数据）。
    </ConfirmDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import logoPng from '../assets/logo.png'
import { store, refreshConnections, openConnection, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import { fmtBytes } from '../lib/format'
import type { ConnDTO, ServerSummary } from '../lib/types'

const showForm = ref(false)
const editing = ref(false)
const testing = ref(false)
// 测试结果统一状态：done 表示已出结果（成功或失败都要有明确提示）
const testState = reactive<{
  done: boolean
  ok: boolean
  message: string
  summary: ServerSummary | null
}>({ done: false, ok: false, message: '', summary: null })
const formHasPassword = ref(false)
const formHasSSHPass = ref(false)

const form = reactive<any>(emptyForm())
const sentinelAddrsText = ref('')

const groupNames = computed(() =>
  Array.from(new Set(store.connections.map((c) => c.group).filter(Boolean))),
)

const grouped = computed(() => {
  const groups = new Map<string, ConnDTO[]>()
  for (const c of store.connections) {
    const g = c.group || '默认分组'
    if (!groups.has(g)) groups.set(g, [])
    groups.get(g)!.push(c)
  }
  return Array.from(groups.entries()).map(([name, items]) => ({ name, items }))
})

function emptyForm() {
  return {
    id: '',
    name: '',
    group: '',
    mode: 'standalone',
    host: '127.0.0.1',
    port: 6379,
    username: '',
    password: '',
    db: 0,
    readOnly: false,
    ssh: { enabled: false, host: '', port: 22, user: 'root', authType: 'password', password: '', keyPath: '', passphrase: '' },
    tls: { enabled: false, certFile: '', keyFile: '', caFile: '', serverName: '', insecureSkipVerify: false },
    sentinel: { master: '', addrs: [], username: '', password: '' },
  }
}

function resetTestState() {
  testState.done = false
  testState.ok = false
  testState.message = ''
  testState.summary = null
}

function startNew() {
  Object.assign(form, emptyForm())
  sentinelAddrsText.value = ''
  editing.value = false
  formHasPassword.value = false
  formHasSSHPass.value = false
  resetTestState()
  showForm.value = true
}

function editConn(c: ConnDTO) {
  Object.assign(form, JSON.parse(JSON.stringify(c)))
  delete form.hasPassword
  delete form.hasSSHPass
  delete form.hasSSHKey
  delete form.health
  delete form.healthError
  delete form.open
  form.password = ''
  form.ssh.password = ''
  form.ssh.passphrase = ''
  form.sentinel.password = ''
  sentinelAddrsText.value = (c.sentinel?.addrs || []).join(', ')
  editing.value = true
  formHasPassword.value = c.hasPassword
  formHasSSHPass.value = c.hasSSHPass
  resetTestState()
  showForm.value = true
}

async function test() {
  testing.value = true
  resetTestState()
  try {
    const r = await api.testConnection(buildPayload())
    testState.done = true
    testState.ok = true
    testState.summary = r
    toast('ok', `连接测试成功：Redis ${r.version}`)
  } catch (e) {
    testState.done = true
    testState.ok = false
    testState.message = errMsg(e)
    toast('err', `连接测试失败：${testState.message}`)
  } finally {
    testing.value = false
  }
}

function buildPayload() {
  const payload = JSON.parse(JSON.stringify(form))
  if (!payload.id) payload.id = crypto.randomUUID()
  payload.sentinel.addrs = sentinelAddrsText.value
    .split(/[,，]/)
    .map((s: string) => s.trim())
    .filter(Boolean)
  return payload
}

async function save() {
  try {
    const c = await api.saveConnection(buildPayload(), false)
    await refreshConnections()
    toast('ok', `连接「${c.name}」已保存`)
    showForm.value = false
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function openConn(c: ConnDTO) {
  try {
    await openConnection(c.id)
  } catch {
    /* toast 已提示 */
  }
}

async function closeConn(c: ConnDTO) {
  await api.closeConnection(c.id)
  await refreshConnections()
  if (store.currentConnId === c.id) {
    store.currentConnId = ''
    store.view = 'connections'
  }
}

// 删除连接确认（应用内对话框，避免依赖 WebView2 原生 confirm）
const delTarget = ref<ConnDTO | null>(null)

function askDelete(c: ConnDTO) {
  delTarget.value = c
}

async function doDelete() {
  const c = delTarget.value
  delTarget.value = null
  if (!c) return
  try {
    await api.deleteConnection(c.id)
    if (store.currentConnId === c.id) {
      store.currentConnId = ''
      store.view = 'connections'
    }
    await refreshConnections()
    toast('ok', '连接已删除')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function pickKey() {
  const f = await api.pickFile('选择 SSH 私钥')
  if (f) form.ssh.keyPath = f
}
async function pickCA() {
  const f = await api.pickFile('选择 CA 证书')
  if (f) form.tls.caFile = f
}
async function pickCert() {
  const f = await api.pickFile('选择客户端证书')
  if (f) form.tls.certFile = f
}
async function pickTLSKey() {
  const f = await api.pickFile('选择客户端私钥')
  if (f) form.tls.keyFile = f
}
</script>

<style scoped>
.connman {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.cm-head { display: flex; align-items: center; gap: 12px; }
.cm-head h2 { margin: 0; font-size: 17px; font-weight: 700; }

.cm-group { margin-bottom: 16px; }
.cm-group-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}
.cm-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 8px;
}
.cm-item.active { border-left: 3px solid var(--red); }
.dot { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; background: var(--text-3); }
.dot.ok { background: var(--green); box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.15); }
.dot.down { background: var(--red); }
.dot.connecting { background: var(--amber); }

.cm-info { cursor: pointer; min-width: 0; }
.cm-name { font-weight: 600; font-size: 14px; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.cm-addr { font-size: 12px; margin-top: 3px; }
.ro-tag { background: rgba(59, 130, 246, 0.12); color: var(--t-hash); }
.mode-tag { background: rgba(168, 85, 247, 0.12); color: var(--t-set); }
.ssh-tag { background: rgba(34, 197, 94, 0.12); color: var(--t-list); }
.tls-tag { background: rgba(6, 182, 212, 0.12); color: var(--t-stream); }

.cm-form { padding: 20px 24px; max-width: 860px; }
.form-title { display: flex; align-items: center; margin-bottom: 16px; }
.form-title h3 { margin: 0; font-size: 15px; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 12px;
}
.f-field { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.f-field label { font-size: 12px; color: var(--text-2); font-weight: 600; }
.btn-row { display: flex; gap: 6px; align-items: center; }
.chk-row { display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 12.5px !important; }
.chk-row input { accent-color: var(--red); }

.form-section {
  border-top: 1px dashed var(--border);
  padding-top: 14px;
  margin-top: 8px;
}
.form-section h4 { margin: 0 0 10px; font-size: 13px; color: var(--text-2); }
.toggle-head {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  user-select: none;
}
.toggle-head:hover { color: var(--text); }
.toggle {
  width: 32px;
  height: 18px;
  border-radius: 9px;
  background: var(--border-strong);
  position: relative;
  transition: background 0.15s;
  flex-shrink: 0;
}
.toggle::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  transition: left 0.15s;
}
.toggle.on { background: var(--red); }
.toggle.on::after { left: 16px; }

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}
.test-result {
  margin-top: 12px;
  padding: 12px 16px;
  font-size: 13px;
}
</style>
