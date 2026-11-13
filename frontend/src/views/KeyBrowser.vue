<template>
  <div class="kb">
    <!-- 工具栏 -->
    <div class="kb-toolbar">
      <div class="filter-group">
        <Icon name="search" :size="14" class="faint" />
        <input
          v-model="pattern"
          class="pattern-input mono"
          placeholder="键名匹配模式，如 user:* （回车生效）"
          @keyup.enter="reload"
          spellcheck="false"
        />
      </div>
      <select v-model="typeFilter" @change="reload" class="type-select">
        <option value="">全部类型</option>
        <option v-for="(m, t) in TYPE_META" :key="t" :value="t">{{ t }}</option>
      </select>
      <button class="btn sm" @click="reload()" title="刷新">
        <Icon name="refresh" :size="14" />
      </button>
      <span class="grow"></span>
      <button class="btn sm primary" @click="showCreate = true" :disabled="conn?.readOnly">
        <Icon name="plus" :size="14" /> 新建键
      </button>
      <button class="btn sm" :disabled="selectedKeys.size === 0 || conn?.readOnly" @click="batchTTL">
        <Icon name="clock" :size="14" /> 批量续期
      </button>
      <button class="btn sm" :disabled="selectedKeys.size === 0 || conn?.readOnly" @click="batchPersist">
        <Icon name="history" :size="14" /> 移除TTL
      </button>
      <button class="btn sm danger-ghost" :disabled="selectedKeys.size === 0 || conn?.readOnly" @click="batchDelete">
        <Icon name="trash" :size="14" /> 删除 ({{ selectedKeys.size }})
      </button>
    </div>

    <!-- 键列表 -->
    <div class="kb-list">
      <div class="list-head">
        <label class="chk">
          <input type="checkbox" :checked="allSelected" @change="toggleAll" />
        </label>
        <span class="col-key">键名</span>
        <span class="col-type">类型</span>
        <span class="col-ttl">TTL</span>
        <span class="col-size">大小</span>
      </div>
      <VirtualList
        v-if="keys.length"
        ref="vlist"
        :items="keys"
        :row-height="rowH"
        :item-key="(k: any) => k.key"
        class="list-body"
      >
        <template #default="{ item }">
          <label class="chk">
            <input
              type="checkbox"
              :checked="selectedKeys.has(item.key)"
              @change="toggleSelect(item.key)"
              @click.stop
            />
          </label>
          <span
            class="col-key mono ellipsis clickable"
            :class="{ active: item.key === currentKey }"
            @click="openKey(item.key)"
            :title="item.key"
          >{{ item.key }}</span>
          <span class="col-type"><TypeBadge :type="item.type" /></span>
          <span class="col-ttl mono" :class="{ expiring: item.ttl > 0 && item.ttl < 60000 }">{{ fmtTTL(item.ttl) }}</span>
          <span class="col-size mono faint">{{ item.size >= 0 ? fmtBytes(item.size) : '-' }}</span>
        </template>
      </VirtualList>
      <div v-else class="empty">
        <Icon name="key" :size="40" />
        <div>{{ loading ? '加载中…' : pattern ? '没有匹配的键' : '该库暂无键' }}</div>
        <button class="btn sm" @click="reload()">刷新</button>
      </div>
      <!-- 分页加载 -->
      <div v-if="keys.length" class="list-foot">
        <span class="faint">已加载 {{ keys.length }} 键{{ cursor > 0 ? `（游标 ${cursor}，继续滚动加载）` : '（全部）' }}</span>
        <button v-if="cursor > 0" class="btn sm" @click="loadMore()">加载更多</button>
      </div>
    </div>

    <!-- 值查看器 + 详情 -->
    <div class="kb-lower">
      <ValueViewer :value-data="valueData" :read-only="!!conn?.readOnly" @refresh="refreshValue" />
      <DetailPanel
        v-if="store.detailVisible && valueData"
        :value-data="valueData"
        :read-only="!!conn?.readOnly"
        @refresh="refreshValue"
        @deleted="onKeyDeleted"
      />
    </div>

    <!-- 新建键 -->
    <Modal :visible="showCreate" title="新建键" width="520px" @close="showCreate = false">
      <div class="form-col">
        <label>键名</label>
        <input v-model="newKey" class="mono" placeholder="如 user:1001:profile" spellcheck="false" />
        <label>类型</label>
        <select v-model="newType">
          <option v-for="t in ['string', 'hash', 'list', 'set', 'zset', 'stream', 'json']" :key="t" :value="t">{{ t }}</option>
        </select>
        <label>初始值</label>
        <textarea v-model="newInitial" rows="4" class="mono" spellcheck="false"></textarea>
        <label>TTL（毫秒，留空表示永久）</label>
        <input v-model="newTTL" type="number" placeholder="永不过期" />
      </div>
      <template #footer>
        <button class="btn" @click="showCreate = false">取消</button>
        <button class="btn primary" @click="doCreate" :disabled="!newKey">创建</button>
      </template>
    </Modal>

    <!-- 危险操作确认 -->
    <ConfirmDialog
      :visible="confirm.visible"
      :title="confirm.title"
      :danger="true"
      :confirm-text="confirm.confirmText"
      @confirm="confirm.onOk?.()"
      @cancel="confirm.visible = false"
    >
      {{ confirm.message }}
    </ConfirmDialog>

    <!-- 批量 TTL 输入 -->
    <Modal :visible="batchTtlVisible" :title="`为 ${selectedKeys.size} 个键设置 TTL`" width="420px" @close="batchTtlVisible = false">
      <div class="form-col">
        <label>TTL（秒）</label>
        <input v-model="batchTtlSec" type="number" min="1" @keyup.enter="doBatchTTL" />
      </div>
      <template #footer>
        <button class="btn" @click="batchTtlVisible = false">取消</button>
        <button class="btn primary" @click="doBatchTTL">应用</button>
      </template>
    </Modal>

    <!-- Diff 预览 -->
    <Modal :visible="diff.visible" title="保存前变更预览（Diff）" width="640px" @close="diff.visible = false">
      <div class="diff-wrap">
        <div class="diff-col">
          <div class="diff-label faint">当前值</div>
          <pre class="code-view diff-old">{{ diff.old }}</pre>
        </div>
        <div class="diff-col">
          <div class="diff-label faint">新值</div>
          <pre class="code-view diff-new">{{ diff.new }}</pre>
        </div>
      </div>
      <template #footer>
        <button class="btn" @click="diff.visible = false">取消</button>
        <button class="btn primary" @click="diff.onOk?.()">确认保存</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import TypeBadge from '../components/TypeBadge.vue'
import VirtualList from '../components/VirtualList.vue'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import ValueViewer from './value/ValueViewer.vue'
import DetailPanel from './value/DetailPanel.vue'
import { store, currentConn, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import { fmtBytes, fmtTTL, TYPE_META } from '../lib/format'
import type { KeyItem, ValueData } from '../lib/types'

const conn = computed(() => currentConn())
const rowH = computed(() => ({ compact: 28, comfortable: 34, spacious: 42 }[store.settings.density] || 34))

const pattern = ref('*')
const typeFilter = ref('')
const keys = ref<KeyItem[]>([])
const cursor = ref(0)
const loading = ref(false)
const selectedKeys = reactive(new Set<string>())
const currentKey = ref('')
const valueData = ref<ValueData | null>(null)
const vlist = ref()

const allSelected = computed(() => keys.value.length > 0 && keys.value.every((k) => selectedKeys.has(k.key)))

function toggleAll() {
  if (allSelected.value) selectedKeys.clear()
  else keys.value.forEach((k) => selectedKeys.add(k.key))
}
function toggleSelect(key: string) {
  if (selectedKeys.has(key)) selectedKeys.delete(key)
  else selectedKeys.add(key)
}

async function reload() {
  if (!conn.value) return
  loading.value = true
  keys.value = []
  cursor.value = 0
  selectedKeys.clear()
  await loadMore(true)
  loading.value = false
}

async function loadMore(reset = false) {
  if (!conn.value) return
  try {
    const resp = await api.scanPage(conn.value.id, store.currentDB, reset ? 0 : cursor.value, pattern.value || '*', typeFilter.value, store.settings.scanCount)
    if (reset) keys.value = []
    keys.value.push(...resp.keys)
    cursor.value = resp.cursor
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function openKey(key: string) {
  currentKey.value = key
  await refreshValue()
}

async function refreshValue() {
  if (!conn.value || !currentKey.value) return
  try {
    valueData.value = await api.getValue(conn.value.id, store.currentDB, currentKey.value)
  } catch (e) {
    valueData.value = null
    toast('err', errMsg(e))
  }
}

function onKeyDeleted() {
  keys.value = keys.value.filter((k) => k.key !== currentKey.value)
  selectedKeys.delete(currentKey.value)
  currentKey.value = ''
  valueData.value = null
}

// ---------- 批量操作 ----------
const batchTtlVisible = ref(false)
const batchTtlSec = ref('3600')

const confirm = reactive<{
  visible: boolean
  title: string
  message: string
  confirmText?: string
  onOk?: () => void
}>({ visible: false, title: '', message: '' })

function batchDelete() {
  const keysArr = Array.from(selectedKeys)
  confirm.visible = true
  confirm.title = `删除 ${keysArr.length} 个键`
  confirm.message = `将删除选中的 ${keysArr.length} 个键。删除前会自动备份到回收站，可随时恢复。`
  confirm.confirmText = undefined
  confirm.onOk = async () => {
    confirm.visible = false
    try {
      const res = await api.deleteKeys(conn.value!.id, store.currentDB, keysArr, true)
      toast('ok', `已删除 ${res.deleted} 个键（回收站 ${res.backedUp} 条，可恢复）`)
      keys.value = keys.value.filter((k) => !selectedKeys.has(k.key))
      selectedKeys.clear()
      if (currentKey.value && keysArr.includes(currentKey.value)) onKeyDeleted()
    } catch (e) {
      toast('err', errMsg(e))
    }
  }
}

function batchTTL() {
  // 应用内输入对话框（避免依赖 WebView2 的原生 prompt）
  batchTtlSec.value = '3600'
  batchTtlVisible.value = true
}

async function doBatchTTL() {
  const sec = parseInt(batchTtlSec.value)
  batchTtlVisible.value = false
  if (isNaN(sec) || sec <= 0) {
    toast('err', '无效的 TTL')
    return
  }
  try {
    const r = await api.setTTL(conn.value!.id, store.currentDB, Array.from(selectedKeys), sec * 1000)
    toast('ok', `已为 ${r.affected} 个键设置 TTL ${sec}s`)
    await refreshValue()
    reload()
  } catch (e) {
    toast('err', errMsg(e))
  }
}

function batchPersist() {
  confirm.visible = true
  confirm.title = '移除 TTL'
  confirm.message = `将移除 ${selectedKeys.size} 个键的过期时间，使其永久存活。`
  confirm.onOk = async () => {
    confirm.visible = false
    try {
      const r = await api.setTTL(conn.value!.id, store.currentDB, Array.from(selectedKeys), -2)
      toast('ok', `已移除 ${r.affected} 个键的 TTL`)
      reload()
    } catch (e) {
      toast('err', errMsg(e))
    }
  }
}

// ---------- 新建键 ----------
const showCreate = ref(false)
const newKey = ref('')
const newType = ref('string')
const newInitial = ref('')
const newTTL = ref('')

async function doCreate() {
  try {
    const ttlMs = newTTL.value ? parseInt(newTTL.value) : -2
    await api.createKey(conn.value!.id, store.currentDB, newKey.value, newType.value, newInitial.value, ttlMs)
    toast('ok', `键 ${newKey.value} 创建成功`)
    showCreate.value = false
    newKey.value = ''
    newInitial.value = ''
    newTTL.value = ''
    reload()
    openKey(newKey.value || '')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

// ---------- Diff 预览（供 ValueViewer 调用） ----------
const diff = reactive<{ visible: boolean; old: string; new: string; onOk?: () => void }>({
  visible: false,
  old: '',
  new: '',
})

function askDiff(oldText: string, newText: string, onOk: () => void) {
  diff.old = oldText
  diff.new = newText
  diff.onOk = () => {
    diff.visible = false
    onOk()
  }
  diff.visible = true
}

provideDiffAPI()

function provideDiffAPI() {
  ;(window as any).__rlAskDiff = askDiff
}

// ---------- 外部事件 ----------
function onSelectPrefix(e: Event) {
  const prefix = (e as CustomEvent).detail as string
  pattern.value = prefix === '' ? '*' : prefix + ':*'
  reload()
}
function onOpenKey(e: Event) {
  const key = (e as CustomEvent).detail as string
  openKey(key)
}

onMounted(() => {
  reload()
  window.addEventListener('redislens:select-prefix', onSelectPrefix)
  window.addEventListener('redislens:open-key', onOpenKey)
})
onBeforeUnmount(() => {
  window.removeEventListener('redislens:select-prefix', onSelectPrefix)
  window.removeEventListener('redislens:open-key', onOpenKey)
})
</script>

<style scoped>
.kb {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}
.kb-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-panel);
  flex-shrink: 0;
}
.filter-group {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 0 10px;
  background: var(--bg-input);
  width: 320px;
}
.filter-group input {
  border: none;
  padding: 6px 0;
  flex: 1;
  font-size: 12.5px;
}
.filter-group input:focus { box-shadow: none; }
.type-select { width: 110px; }

.kb-list {
  height: 42%;
  display: flex;
  flex-direction: column;
  border-bottom: 1px solid var(--border);
  background: var(--bg-panel);
  min-height: 0;
}
.list-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  height: 32px;
  border-bottom: 1px solid var(--border);
  font-size: 11.5px;
  color: var(--text-3);
  font-weight: 600;
  flex-shrink: 0;
}
.list-body { flex: 1; padding: 0 6px; }
.list-body :deep(.vlist-row) {
  gap: 8px;
  padding: 0 8px;
  border-radius: var(--radius-sm);
  font-size: var(--fs);
  cursor: default;
}
.list-body :deep(.vlist-row:hover) { background: var(--bg-hover); }
.col-key { flex: 1; min-width: 0; }
.col-key.clickable { cursor: pointer; padding: 2px 6px; border-radius: 4px; }
.col-key.clickable:hover { background: var(--bg-active); }
.col-key.active { color: var(--red); font-weight: 600; background: var(--accent-soft); }
.col-type { width: 76px; flex-shrink: 0; }
.col-ttl { width: 90px; flex-shrink: 0; text-align: right; }
.col-ttl.expiring { color: var(--amber); font-weight: 600; }
.col-size { width: 80px; flex-shrink: 0; text-align: right; }
.chk { display: flex; align-items: center; flex-shrink: 0; }
.chk input { accent-color: var(--red); width: 14px; height: 14px; }

.list-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 14px;
  border-top: 1px solid var(--border);
  font-size: 11.5px;
  flex-shrink: 0;
}

.kb-lower {
  flex: 1;
  display: flex;
  min-height: 0;
}
.kb-lower > :first-child { flex: 1; min-width: 0; }

.form-col { display: flex; flex-direction: column; gap: 6px; }
.form-col label { font-size: 12px; color: var(--text-2); font-weight: 600; margin-top: 6px; }

.diff-wrap { display: flex; gap: 12px; max-height: 46vh; }
.diff-col { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.diff-label { font-size: 11.5px; font-weight: 600; }
.diff-old, .diff-new {
  margin: 0;
  padding: 10px;
  border-radius: var(--radius-sm);
  max-height: 38vh;
  overflow: auto;
  font-size: 12px;
}
.diff-old { background: var(--danger-bg); }
.diff-new { background: rgba(22, 163, 74, 0.09); }
</style>
