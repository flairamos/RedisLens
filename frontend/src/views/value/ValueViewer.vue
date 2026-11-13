<template>
  <div class="vv" v-if="vd">
    <!-- 类型头 -->
    <div class="vv-head">
      <TypeBadge :type="vd.type" />
      <span class="mono ellipsis key-title">{{ vd.key }}</span>
      <span class="grow"></span>
      <span class="faint mono" style="font-size: 11.5px">
        {{ vd.elements.toLocaleString() }} {{ vd.type === 'string' || vd.type === 'json' ? 'bytes' : 'elements' }}
        <template v-if="vd.encoding"> · {{ vd.encoding }}</template>
      </span>
      <button class="btn sm ghost" @click="emit('refresh')" title="刷新值">
        <Icon name="refresh" :size="14" />
      </button>
    </div>

    <!-- String / JSON -->
    <div v-if="vd.type === 'string' || vd.type === 'json'" class="vv-body">
      <div class="editor-bar">
        <button
          v-if="vd.isJson"
          class="btn sm"
          :class="{ ghost: !pretty }"
          @click="pretty = !pretty"
        >{{ pretty ? '原始' : '格式化' }}</button>
        <span v-if="vd.binary" class="tag" style="background: var(--danger-bg); color: var(--red)">二进制（Base64 展示）</span>
        <span class="grow"></span>
        <button class="btn sm primary" :disabled="readOnly || !edited" @click="saveString">
          <Icon name="save" :size="13" /> 保存
        </button>
      </div>
      <textarea
        v-model="editText"
        class="str-editor mono"
        :class="{ binary: vd.binary }"
        spellcheck="false"
        :disabled="readOnly"
      ></textarea>
    </div>

    <!-- Hash -->
    <div v-else-if="vd.type === 'hash'" class="vv-body">
      <div class="editor-bar">
        <button class="btn sm" :disabled="readOnly" @click="startAdd('hash')"><Icon name="plus" :size="13" /> 字段</button>
        <span class="grow"></span>
        <span class="faint">双击值编辑</span>
      </div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead><tr><th style="width: 32%">字段</th><th>值</th><th style="width: 64px"></th></tr></thead>
          <tbody>
            <tr v-for="f in vd.hash" :key="f.field">
              <td class="mono">{{ f.field }}</td>
              <td
                class="mono editable"
                :title="f.value"
                @dblclick="startEditHash(f)"
              >
                <span v-if="editing?.kind === 'hash' && editing.field === f.field" class="edit-box">
                  <input v-model="editBuf" @keyup.enter="saveEdit" @keyup.esc="cancelEdit" :ref="el => focusInput(el)" spellcheck="false" />
                </span>
                <template v-else>{{ f.value }}</template>
              </td>
              <td>
                <button class="btn sm ghost" :disabled="readOnly" @click="delHash(f)" title="删除字段">
                  <Icon name="trash" :size="13" style="color: var(--red)" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- List -->
    <div v-else-if="vd.type === 'list'" class="vv-body">
      <div class="editor-bar">
        <button class="btn sm" :disabled="readOnly" @click="pushList('left')"><Icon name="plus" :size="13" /> 左侧插入</button>
        <button class="btn sm" :disabled="readOnly" @click="pushList('right')"><Icon name="plus" :size="13" /> 右侧追加</button>
        <span class="grow"></span>
        <span class="faint">双击元素编辑 · 定位：{{ jumpIndex === null ? '-' : jumpIndex }}</span>
        <input v-model.number="jumpIndex" type="number" style="width: 80px" placeholder="下标" @keyup.enter="doJump" />
      </div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead><tr><th style="width: 90px">下标</th><th>元素</th><th style="width: 64px"></th></tr></thead>
          <tbody>
            <tr v-for="it in displayList" :key="it.index">
              <td class="mono faint">{{ it.index }}</td>
              <td class="mono editable" @dblclick="startEditList(it)">
                <span v-if="editing?.kind === 'list' && editing.index === it.index" class="edit-box">
                  <input v-model="editBuf" @keyup.enter="saveEdit" @keyup.esc="cancelEdit" :ref="el => focusInput(el)" spellcheck="false" />
                </span>
                <template v-else>{{ it.value }}</template>
              </td>
              <td>
                <button class="btn sm ghost" :disabled="readOnly" @click="delList(it.index)" title="删除元素">
                  <Icon name="trash" :size="13" style="color: var(--red)" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Set -->
    <div v-else-if="vd.type === 'set'" class="vv-body">
      <div class="editor-bar">
        <button class="btn sm" :disabled="readOnly" @click="startAdd('set')"><Icon name="plus" :size="13" /> 成员</button>
      </div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead><tr><th>成员</th><th style="width: 64px"></th></tr></thead>
          <tbody>
            <tr v-for="m in vd.set" :key="m">
              <td class="mono">{{ m }}</td>
              <td>
                <button class="btn sm ghost" :disabled="readOnly" @click="delSet(m)">
                  <Icon name="trash" :size="13" style="color: var(--red)" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ZSet -->
    <div v-else-if="vd.type === 'zset'" class="vv-body">
      <div class="editor-bar">
        <button class="btn sm" :disabled="readOnly" @click="startAdd('zset')"><Icon name="plus" :size="13" /> 成员</button>
      </div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead><tr><th style="width: 70px">排名</th><th>成员</th><th style="width: 130px">分数</th><th style="width: 64px"></th></tr></thead>
          <tbody>
            <tr v-for="z in vd.zset" :key="z.rank">
              <td class="mono faint">#{{ z.rank + 1 }}</td>
              <td class="mono">{{ z.member }}</td>
              <td
                class="mono editable"
                @dblclick="startEditZset(z)"
              >
                <span v-if="editing?.kind === 'zset' && editing.field === z.member" class="edit-box">
                  <input v-model="editBuf" @keyup.enter="saveEdit" @keyup.esc="cancelEdit" :ref="el => focusInput(el)" spellcheck="false" />
                </span>
                <template v-else>{{ z.score }}</template>
              </td>
              <td>
                <button class="btn sm ghost" :disabled="readOnly" @click="delZset(z.member)">
                  <Icon name="trash" :size="13" style="color: var(--red)" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Stream -->
    <div v-else-if="vd.type === 'stream'" class="vv-body">
      <div class="editor-bar">
        <button class="btn sm" :disabled="readOnly" @click="startAdd('stream')"><Icon name="plus" :size="13" /> 消息</button>
      </div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead><tr><th style="width: 200px">ID</th><th>字段</th><th style="width: 64px"></th></tr></thead>
          <tbody>
            <tr v-for="e in vd.stream" :key="e.id">
              <td class="mono faint">{{ e.id }}</td>
              <td class="mono">{{ formatStreamFields(e.fields) }}</td>
              <td>
                <button class="btn sm ghost" :disabled="readOnly" @click="delStream(e.id)">
                  <Icon name="trash" :size="13" style="color: var(--red)" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 其他类型 -->
    <div v-else class="empty">
      <Icon name="alert" :size="36" />
      <div>暂不支持类型「{{ vd.type }}」的可视化编辑</div>
    </div>

    <!-- 新增成员对话框 -->
    <Modal :visible="add.visible" :title="addTitle" @close="add.visible = false">
      <div class="form-col">
        <template v-if="add.kind === 'hash'">
          <label>字段</label>
          <input v-model="add.field" class="mono" spellcheck="false" />
        </template>
        <template v-else-if="add.kind === 'zset'">
          <label>成员</label>
          <input v-model="add.field" class="mono" spellcheck="false" />
          <label>分数</label>
          <input v-model.number="add.score" type="number" />
        </template>
        <template v-else-if="add.kind === 'stream'">
          <label>消息 ID（留空自动生成）</label>
          <input v-model="add.id" class="mono" placeholder="*" spellcheck="false" />
          <label>字段（每行 key=value）</label>
          <textarea v-model="add.value" rows="4" class="mono" spellcheck="false"></textarea>
        </template>
        <template v-else>
          <label>值</label>
          <textarea v-model="add.value" rows="4" class="mono" spellcheck="false"></textarea>
        </template>
      </div>
      <template #footer>
        <button class="btn" @click="add.visible = false">取消</button>
        <button class="btn primary" @click="doAdd">添加</button>
      </template>
    </Modal>
  </div>

  <div v-else class="empty vv-empty">
    <Icon name="key" :size="44" />
    <div>在上方键列表中选择一个键查看值</div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import Icon from '../../components/Icon.vue'
import TypeBadge from '../../components/TypeBadge.vue'
import Modal from '../../components/Modal.vue'
import { currentConn, store, toast } from '../../lib/store'
import { api, errMsg } from '../../lib/api'
import { highlightJSON, prettyJSON } from '../../lib/format'
import type { ValueData } from '../../lib/types'

const props = defineProps<{ valueData: ValueData | null; readOnly: boolean }>()
const emit = defineEmits<{ (e: 'refresh'): void }>()

const vd = computed(() => props.valueData)

// ---------- 字符串编辑 ----------
const editText = ref('')
const pretty = ref(true)
const edited = ref(false)

watch(
  () => vd.value?.key,
  () => {
    editText.value = displayText()
    edited.value = false
  },
)
watch(
  () => editText.value,
  () => {
    if (vd.value && editText.value !== displayText()) edited.value = true
  },
)

const displayJson = computed(() => {
  if (vd.value?.isJson && pretty.value) return prettyJSON(vd.value.str || '')
  return vd.value?.str || ''
})
const editingJson = computed(() => {
  if (!vd.value?.isJson) return editText.value
  try {
    return JSON.stringify(JSON.parse(editText.value))
  } catch {
    return editText.value
  }
})

function displayText(): string {
  return displayJson.value
}

async function saveString() {
  const conn = currentConn()
  if (!conn || !vd.value) return
  const go = () =>
    api
      .setString(conn.id, store.currentDB, vd.value!.key, vd.value!.isJson ? editingJson.value : editText.value, vd.value!.binary, -1)
      .then(() => {
        toast('ok', '已保存')
        edited.value = false
        emit('refresh')
      })
      .catch((e) => toast('err', errMsg(e)))
  // PRD：保存前给出 Diff 预览
  const ask = (window as any).__rlAskDiff as (o: string, n: string, cb: () => void) => void
  if (ask) {
    ask(vd.value.str || '', vd.value.isJson ? prettyJSON(editingJson.value) : editText.value, go)
  } else {
    go()
  }
}

// ---------- 通用行内编辑 ----------
const editing = ref<{ kind: string; field?: string; index?: number } | null>(null)
const editBuf = ref('')

function focusInput(el: any) {
  if (el) (el as HTMLInputElement).focus()
}

function startEditHash(f: { field: string; value: string }) {
  if (props.readOnly) return
  editing.value = { kind: 'hash', field: f.field }
  editBuf.value = f.value
}
function startEditList(it: { index: number; value: string }) {
  if (props.readOnly) return
  editing.value = { kind: 'list', index: it.index }
  editBuf.value = it.value
}
function startEditZset(z: { member: string; score: number }) {
  if (props.readOnly) return
  editing.value = { kind: 'zset', field: z.member }
  editBuf.value = String(z.score)
}

function cancelEdit() {
  editing.value = null
}

async function saveEdit() {
  const conn = currentConn()
  if (!conn || !editing.value || !vd.value) return
  try {
    if (editing.value.kind === 'hash') {
      await api.hashSetField(conn.id, store.currentDB, vd.value.key, editing.value.field!, editBuf.value, false)
    } else if (editing.value.kind === 'list') {
      await api.listSetIndex(conn.id, store.currentDB, vd.value.key, editing.value.index!, editBuf.value, false)
    } else if (editing.value.kind === 'zset') {
      const score = parseFloat(editBuf.value)
      if (isNaN(score)) throw new Error('无效的分数')
      await api.zsetUpsert(conn.id, store.currentDB, vd.value.key, editing.value.field!, score, false)
    }
    editing.value = null
    toast('ok', '已保存')
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

// ---------- 新增 ----------
const add = reactive({
  visible: false,
  kind: '',
  field: '',
  value: '',
  score: 1,
  id: '',
  side: 'right',
})
const addTitle = computed(() => (({ hash: '新增字段', set: '新增成员', zset: '新增成员', stream: '新增消息', list: '插入元素' } as Record<string, string>)[add.kind] || '新增'))

function startAdd(kind: string) {
  add.visible = true
  add.kind = kind
  add.field = ''
  add.value = ''
  add.score = 1
  add.id = ''
  add.side = 'right'
}

function pushList(side: string) {
  startAdd('list')
  add.side = side
}

async function doAdd() {
  const conn = currentConn()
  if (!conn || !vd.value) return
  try {
    if (add.kind === 'hash') {
      await api.hashSetField(conn.id, store.currentDB, vd.value.key, add.field, add.value, false)
    } else if (add.kind === 'set') {
      await api.setAddMembers(conn.id, store.currentDB, vd.value.key, add.value.split('\n').map((s) => s.trim()).filter(Boolean))
    } else if (add.kind === 'zset') {
      await api.zsetUpsert(conn.id, store.currentDB, vd.value.key, add.field, add.score, false)
    } else if (add.kind === 'stream') {
      const fields: Record<string, string> = {}
      for (const line of add.value.split('\n')) {
        const idx = line.indexOf('=')
        if (idx > 0) fields[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
      }
      await api.streamAdd(conn.id, store.currentDB, vd.value.key, add.id || '*', fields)
    } else if (add.kind === 'list') {
      await api.listPush(conn.id, store.currentDB, vd.value.key, add.side, add.value, false)
    }
    add.visible = false
    toast('ok', '已添加')
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

// ---------- 删除 ----------
async function delHash(f: { field: string }) {
  const conn = currentConn()
  if (!conn || !vd.value) return
  try {
    await api.hashDelFields(conn.id, store.currentDB, vd.value.key, [f.field])
    toast('ok', `字段 ${f.field} 已删除`)
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}
async function delList(index: number) {
  const conn = currentConn()
  if (!conn || !vd.value) return
  try {
    await api.listRemoveIndex(conn.id, store.currentDB, vd.value.key, index)
    toast('ok', `元素 [${index}] 已删除`)
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}
async function delSet(m: string) {
  const conn = currentConn()
  if (!conn || !vd.value) return
  try {
    await api.setRemoveMembers(conn.id, store.currentDB, vd.value.key, [m])
    toast('ok', '成员已删除')
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}
async function delZset(m: string) {
  const conn = currentConn()
  if (!conn || !vd.value) return
  try {
    await api.zsetRemoveMembers(conn.id, store.currentDB, vd.value.key, [m])
    toast('ok', '成员已删除')
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}
async function delStream(id: string) {
  const conn = currentConn()
  if (!conn || !vd.value) return
  try {
    await api.streamDelete(conn.id, store.currentDB, vd.value.key, [id])
    toast('ok', '消息已删除')
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

// ---------- List 定位跳转 ----------
const jumpIndex = ref<number | null>(null)
const displayList = computed(() => {
  if (jumpIndex.value === null || isNaN(jumpIndex.value)) return vd.value?.list || []
  const target = jumpIndex.value
  const list = vd.value?.list || []
  const idx = list.findIndex((i) => i.index === target)
  if (idx < 0) return list
  const start = Math.max(0, Math.min(idx - 3, list.length - 10))
  return list.slice(start, start + 10)
})
function doJump() {
  if (jumpIndex.value !== null && vd.value?.list) {
    const found = vd.value.list.find((i) => i.index === jumpIndex.value)
    if (!found) toast('info', '该下标不在已加载范围内（首页 500 条）')
  }
}

function formatStreamFields(fields: Record<string, string>): string {
  return Object.entries(fields)
    .map(([k, v]) => `${k}=${v}`)
    .join('  ')
}
</script>

<style scoped>
.vv {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  border-right: 1px solid var(--border);
  background: var(--bg-panel);
}
.vv-empty { height: 100%; border-right: 1px solid var(--border); }
.vv-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.key-title { font-size: 13px; font-weight: 600; max-width: 50%; }
.vv-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 10px 14px 14px;
}
.editor-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-shrink: 0;
}
.str-editor {
  flex: 1;
  resize: none;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre;
  overflow: auto;
}
.str-editor.binary { color: var(--text-2); }
.tbl-wrap { flex: 1; overflow: auto; min-height: 0; }
.editable { cursor: cell; }
.edit-box input { width: 100%; min-width: 200px; padding: 3px 8px; font-family: var(--font-mono); font-size: 12px; }
.form-col { display: flex; flex-direction: column; gap: 6px; }
.form-col label { font-size: 12px; color: var(--text-2); font-weight: 600; margin-top: 6px; }
</style>
