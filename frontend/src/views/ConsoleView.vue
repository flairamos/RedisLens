<template>
  <div class="console">
    <!-- 会话头 -->
    <div class="cs-head">
      <Icon name="terminal" :size="15" />
      <span class="cs-title">命令行控制台</span>
      <span class="faint mono" style="font-size: 11.5px">{{ conn?.name }} · DB{{ store.currentDB }}</span>
      <span class="grow"></span>
      <button class="btn sm ghost" @click="clearOut"><Icon name="trash" :size="13" /> 清屏</button>
      <button class="btn sm ghost" @click="histOpen = !histOpen">
        <Icon name="history" :size="13" /> 历史 ({{ history.length }})
      </button>
    </div>

    <!-- 输出区 -->
    <div class="cs-out" ref="outEl">
      <div v-for="(row, i) in rows" :key="i" class="cs-row" :class="row.kind">
        <span v-if="row.time" class="cs-time faint mono">{{ row.time }}</span>
        <span v-if="row.kind === 'cmd'" class="cs-prompt">redis&gt;</span>
        <pre class="cs-text mono">{{ row.text }}</pre>
      </div>
      <div v-if="rows.length === 0" class="cs-welcome faint">
        <p>已连接 <b>{{ conn?.name }}</b>。输入 Redis 命令回车执行。</p>
        <p>危险命令（FLUSHALL / CONFIG / SHUTDOWN 等）将强制拦截并要求二次确认。</p>
        <p class="mono">Tab 补全 · ↑↓ 历史 · Ctrl+K 全局搜索</p>
      </div>
    </div>

    <!-- 输入区 -->
    <div class="cs-input-wrap">
      <div class="cs-input-row">
        <span class="cs-prompt mono">redis&gt;</span>
        <div class="cs-input-box">
          <!-- 高亮覆盖层：位于输入文字下方（z-index 1），随输入横向滚动同步 -->
          <pre
            v-if="input && !composing"
            ref="hlEl"
            class="cs-highlight mono"
            v-html="highlightCmd(input)"
          ></pre>
          <input
            ref="inputEl"
            v-model="input"
            class="cs-input mono"
            :class="{ composing }"
            spellcheck="false"
            autocomplete="off"
            @keydown="onKey"
            @input="syncScroll"
            @scroll="syncScroll"
            @compositionstart="composing = true"
            @compositionend="onCompositionEnd"
            placeholder="输入命令…"
          />
        </div>
        <button class="btn sm primary" @click="exec(false)" :disabled="!input.trim()">执行</button>
      </div>
      <!-- 补全下拉 -->
      <div v-if="completions.length" class="cs-complete card">
        <button
          v-for="(c, i) in completions"
          :key="c.text"
          class="cs-c-item"
          :class="{ active: i === compIdx }"
          @mouseenter="compIdx = i"
          @click="applyCompletion(c)"
        >
          <span class="mono cs-c-text">{{ c.text }}</span>
          <span class="faint" style="font-size: 11px">{{ c.synop }}</span>
          <span v-if="c.danger" class="tag" style="background: var(--danger-bg); color: var(--red)">危险</span>
        </button>
        <div class="cs-c-foot faint">Tab 采纳 · Esc 关闭</div>
      </div>
    </div>

    <!-- 历史面板 -->
    <Modal :visible="histOpen" title="命令历史" width="560px" @close="histOpen = false">
      <div class="hist-list">
        <button v-for="(h, i) in history" :key="i" class="hist-item mono" @click="useHistory(h)">
          {{ h }}
        </button>
        <div v-if="!history.length" class="empty faint">暂无历史</div>
      </div>
      <template #footer>
        <button class="btn danger-ghost" @click="clearHistory">清空历史</button>
        <span class="grow"></span>
        <button class="btn" @click="histOpen = false">关闭</button>
      </template>
    </Modal>

    <!-- 危险命令二次确认 -->
    <ConfirmDialog
      :visible="dangerConfirm.visible"
      :title="`危险命令：${dangerConfirm.op}`"
      :danger="true"
      :require-text="dangerConfirm.op"
      confirm-text="我已知晓风险，继续执行"
      @confirm="doConfirmed"
      @cancel="dangerConfirm.visible = false"
    >
      {{ dangerConfirm.reason }}。该操作将直接作用于
      <b>{{ conn?.name }}</b>，请确认目标实例正确。
      <div class="mono cs-danger-cmd">{{ dangerConfirm.line }}</div>
    </ConfirmDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { store, currentConn, toast } from '../lib/store'
import { api, errMsg, onEvent } from '../lib/api'
import type { CommandHelp } from '../lib/types'

interface Row {
  kind: 'cmd' | 'out' | 'err' | 'warn' | 'ok'
  text: string
  time: string
}

const conn = computed(() => currentConn())
const rows = ref<Row[]>([])
const input = ref('')
const inputEl = ref<HTMLInputElement>()
const hlEl = ref<HTMLElement>()
const outEl = ref<HTMLElement>()
const histOpen = ref(false)
// 中文输入法组合期间：显示原色文字、隐藏高亮覆盖层（组合文本对透明输入不可见）
const composing = ref(false)

// 长命令横向滚动时同步覆盖层位置
function syncScroll() {
  if (hlEl.value && inputEl.value) {
    hlEl.value.scrollLeft = inputEl.value.scrollLeft
  }
}

function onCompositionEnd() {
  composing.value = false
  syncScroll()
}

// 历史（localStorage 按连接保存）
const histKey = computed(() => `redislens.hist.${store.currentConnId}`)
const history = ref<string[]>([])
watch(histKey, (k) => {
  history.value = JSON.parse(localStorage.getItem(k) || '[]')
}, { immediate: true })

function useHistory(h: string) {
  input.value = h
  histOpen.value = false
  inputEl.value?.focus()
}
function clearHistory() {
  history.value = []
  localStorage.removeItem(histKey.value)
}
function clearOut() {
  rows.value = []
}

// 命令表
const cmdList = ref<CommandHelp[]>([])
onMounted(async () => {
  cmdList.value = await api.commandHelpList()
})

// 高亮：危险命令红色 / 读命令蓝色 / 字符串绿色 / 数字橙色
function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function highlightCmd(line: string): string {
  const tokens = line.match(/("[^"]*"|'[^']*'|\S+)/g) || []
  return tokens
    .map((tk) => {
      const plain = tk.replace(/^["']|["']$/g, '')
      const upper = plain.toUpperCase()
      if (/^["']/.test(tk)) return `<span class="s-str">${escapeHtml(tk)}</span>`
      if (/^-?\d+(\.\d+)?$/.test(tk)) return `<span class="s-num">${escapeHtml(tk)}</span>`
      const cmd = cmdList.value.find((c) => c.name === upper)
      if (cmd) {
        return `<span class="${cmd.danger ? 's-danger' : 's-cmd'}">${escapeHtml(tk)}</span>`
      }
      return `<span>${escapeHtml(tk)}</span>`
    })
    .join(' ')
}

// 补全
const completions = ref<Array<{ text: string; synop: string; danger: boolean }>>([])
const compIdx = ref(0)
let keyDebounce = 0

function buildCompletions() {
  const line = input.value
  const tokens = line.split(/\s+/)
  compIdx.value = 0
  if (tokens.length <= 1 && !line.endsWith(' ')) {
    // 命令补全
    const prefix = (tokens[0] || '').toUpperCase()
    if (!prefix) {
      completions.value = []
      return
    }
    completions.value = cmdList.value
      .filter((c) => c.name.startsWith(prefix))
      .slice(0, 10)
      .map((c) => ({ text: c.name, synop: c.synop, danger: c.danger }))
    return
  }
  // 键名补全
  const last = tokens[tokens.length - 1]
  if (!last || !conn.value) {
    completions.value = []
    return
  }
  clearTimeout(keyDebounce)
  keyDebounce = window.setTimeout(async () => {
    try {
      const keys = await api.completeKeys(conn.value!.id, store.currentDB, last)
      completions.value = keys.slice(0, 10).map((k) => ({ text: k, synop: '键名', danger: false }))
    } catch {
      completions.value = []
    }
  }, 200)
}

watch(input, buildCompletions)

function applyCompletion(c: { text: string }) {
  const tokens = input.value.split(/\s+/)
  if (tokens.length <= 1 && !input.value.endsWith(' ')) {
    input.value = c.text + ' '
  } else {
    tokens[tokens.length - 1] = c.text
    input.value = tokens.join(' ')
  }
  completions.value = []
  inputEl.value?.focus()
}

// 历史导航
const histIdx = ref(-1)
let draft = ''

async function onKey(e: KeyboardEvent) {
  if (completions.value.length) {
    if (e.key === 'Tab') {
      e.preventDefault()
      applyCompletion(completions.value[compIdx.value])
      return
    }
    if (e.key === 'ArrowDown' && compIdx.value < completions.value.length - 1) {
      e.preventDefault()
      compIdx.value++
      return
    }
    if (e.key === 'ArrowUp' && compIdx.value > 0) {
      e.preventDefault()
      compIdx.value--
      return
    }
    if (e.key === 'Escape') {
      completions.value = []
      return
    }
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    await exec(false)
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (histIdx.value === -1) draft = input.value
    if (histIdx.value < history.value.length - 1) {
      histIdx.value++
      input.value = history.value[history.value.length - 1 - histIdx.value]
    }
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (histIdx.value > 0) {
      histIdx.value--
      input.value = history.value[history.value.length - 1 - histIdx.value]
    } else {
      histIdx.value = -1
      input.value = draft
    }
    return
  }
  if (e.ctrlKey && e.key.toLowerCase() === 'l') {
    e.preventDefault()
    clearOut()
  }
}

const now = () => new Date().toLocaleTimeString('zh-CN', { hour12: false })

// 危险命令确认流程
const dangerConfirm = reactive({ visible: false, op: '', reason: '', line: '' })
let pendingLine = ''

async function exec(confirm: boolean) {
  const c = conn.value
  const line = input.value.trim()
  if (!c || !line) return
  rows.value.push({ kind: 'cmd', text: line, time: now() })
  try {
    const res = await api.execCommand(c.id, line, confirm)
    if (res.blocked) {
      if (res.needConfirm) {
        pendingLine = line
        dangerConfirm.op = res.op || ''
        dangerConfirm.reason = res.reason || ''
        dangerConfirm.line = line
        dangerConfirm.visible = true
        rows.value.push({ kind: 'warn', text: `⛔ 已拦截：${res.reason}`, time: now() })
      } else {
        rows.value.push({ kind: 'warn', text: `⛔ ${res.reason}`, time: now() })
      }
    } else if (res.ok) {
      rows.value.push({ kind: 'out', text: res.reply, time: now() })
    } else {
      rows.value.push({ kind: 'err', text: res.reply, time: now() })
    }
  } catch (e) {
    rows.value.push({ kind: 'err', text: errMsg(e), time: now() })
  }
  // 记录历史
  if (!confirm) {
    history.value.push(line)
    if (history.value.length > 200) history.value.shift()
    localStorage.setItem(histKey.value, JSON.stringify(history.value))
    histIdx.value = -1
  }
  input.value = ''
  completions.value = []
  await nextTick()
  if (outEl.value) outEl.value.scrollTop = outEl.value.scrollHeight
}

function doConfirmed() {
  dangerConfirm.visible = false
  const line = pendingLine
  pendingLine = ''
  if (line) {
    input.value = line
    exec(true)
  }
}

// 外部提示（全局搜索跳转）
let offHint: (() => void) | null = null
onMounted(() => {
  offHint = onEvent('redislens:console-hint', () => {}) as any
  inputEl.value?.focus()
})
onBeforeUnmount(() => offHint?.())
</script>

<style scoped>
.console {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-panel);
  position: relative;
}
.cs-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.cs-title { font-weight: 600; font-size: 14px; }

.cs-out {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
  min-height: 0;
}
.cs-row { display: flex; gap: 10px; margin-bottom: 2px; align-items: baseline; }
.cs-time { font-size: 10.5px; flex-shrink: 0; width: 66px; }
.cs-prompt { color: var(--red); font-weight: 700; flex-shrink: 0; font-size: 12.5px; }
.cs-text {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
  flex: 1;
  min-width: 0;
}
.cs-row.cmd .cs-text { color: var(--text); font-weight: 600; }
.cs-row.out .cs-text { color: var(--text-2); }
.cs-row.err .cs-text { color: var(--red); }
.cs-row.warn .cs-text { color: var(--amber); }
.cs-welcome { font-size: 13px; line-height: 1.8; }
.cs-welcome p { margin: 4px 0; }

.cs-input-wrap {
  position: relative;
  border-top: 1px solid var(--border);
  padding: 10px 16px 14px;
  flex-shrink: 0;
  background: var(--bg);
}
.cs-input-row { display: flex; align-items: center; gap: 10px; }
/* 容器承载边框与背景；内部为"高亮层(下) + 透明输入(上)"的编辑器覆盖层结构 */
.cs-input-box {
  position: relative;
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  transition: border-color 0.15s;
}
.cs-input-box:focus-within { border-color: var(--accent); }
/* 高亮覆盖层：必须位于输入框之下（z-index 1），输入框透明背景让其可见 */
.cs-highlight {
  position: absolute;
  inset: 0;
  margin: 0;
  padding: 7px 12px;
  font-size: 13px;
  line-height: 1.5;
  pointer-events: none;
  white-space: pre;
  overflow: hidden;
  color: var(--text);
  z-index: 1;
}
/* v-html 注入的 span 无 scoped 属性，用 :deep() 穿透 */
.cs-highlight :deep(.s-cmd) { color: var(--t-hash); font-weight: 700; }
.cs-highlight :deep(.s-danger) { color: var(--red); font-weight: 700; }
.cs-highlight :deep(.s-str) { color: var(--t-list); }
.cs-highlight :deep(.s-num) { color: var(--t-zset); }
/* 输入层：透明背景（露出高亮层）+ 透明文字 + 可见光标 */
.cs-input {
  position: relative;
  z-index: 2;
  width: 100%;
  border: none;
  background: transparent;
  color: transparent;
  caret-color: var(--text);
  font-size: 13px;
  padding: 7px 12px;
}
.cs-input:focus { box-shadow: none; }
/* 中文输入法组合期间恢复可见文字 */
.cs-input.composing { color: var(--text); }
.cs-input::placeholder { color: var(--text-3); }

.cs-complete {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 46px;
  width: 460px;
  max-height: 280px;
  overflow: auto;
  padding: 4px;
  z-index: 30;
}
.cs-c-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  text-align: left;
}
.cs-c-item.active { background: var(--bg-hover); }
.cs-c-text { font-size: 12.5px; color: var(--text); font-weight: 600; }
.cs-c-foot { padding: 6px 10px; font-size: 11px; text-align: right; }

.hist-list { display: flex; flex-direction: column; gap: 4px; max-height: 50vh; overflow: auto; }
.hist-item {
  text-align: left;
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  color: var(--text);
}
.hist-item:hover { background: var(--bg-hover); }
.cs-danger-cmd {
  margin-top: 10px;
  padding: 10px;
  background: var(--danger-bg);
  color: var(--red);
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  word-break: break-all;
}
</style>
