<template>
  <Teleport to="body">
    <div v-if="open" class="gs-mask" @click.self="emit('close')">
      <div class="gs-panel card">
        <div class="gs-input-row">
          <Icon name="search" :size="16" class="faint" />
          <input
            ref="inputEl"
            v-model="query"
            placeholder="搜索键名、命令或功能模块…"
            @keydown="onKey"
            autofocus
          />
          <kbd>ESC</kbd>
        </div>
        <div class="gs-results">
          <template v-for="(g, gi) in groups" :key="g.label">
            <div class="gs-group-label">{{ g.label }}</div>
            <button
              v-for="(r, ri) in g.items"
              :key="g.label + ri"
              class="gs-item"
              :class="{ active: flatIndex(gi, ri) === activeIdx }"
              @mouseenter="activeIdx = flatIndex(gi, ri)"
              @click="r.run()"
            >
              <Icon :name="r.icon" :size="14" :style="{ color: r.color }" />
              <span class="gs-main">{{ r.title }}</span>
              <span class="gs-sub faint ellipsis">{{ r.subtitle }}</span>
            </button>
          </template>
          <div v-if="!query" class="gs-hint faint">
            输入键名前缀搜索键；输入命令名查看用法；输入模块名直接跳转。↑↓ 选择，回车执行。
          </div>
          <div v-if="query && groups.length === 0" class="gs-hint faint">无匹配结果</div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import Icon from './Icon.vue'
import { store, currentConn, switchView, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import { TYPE_META } from '../lib/format'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

interface Result {
  title: string
  subtitle: string
  icon: string
  color?: string
  run: () => void
}

const query = ref('')
const inputEl = ref<HTMLInputElement>()
const keyResults = ref<Result[]>([])
const activeIdx = ref(0)

const commands: Result[] = []
let cmdLoaded = false
async function loadCommands() {
  if (cmdLoaded) return
  const list = await api.commandHelpList()
  for (const c of list) {
    commands.push({
      title: c.name,
      subtitle: c.synop + (c.danger ? ' ⚠ 危险' : ''),
      icon: 'terminal',
      color: c.danger ? 'var(--red)' : 'var(--t-hash)',
      run: () => {
        store.view = 'console'
        window.dispatchEvent(new CustomEvent('redislens:console-hint', { detail: c.name + ' ' }))
        emit('close')
      },
    })
  }
  cmdLoaded = true
}

const modules: Result[] = [
  { title: '数据浏览', subtitle: '键空间浏览与编辑', icon: 'key', run: () => { switchView('browser'); emit('close') } },
  { title: '监控仪表盘', subtitle: '核心指标曲线', icon: 'activity', run: () => { switchView('dashboard'); emit('close') } },
  { title: '命令行控制台', subtitle: '交互式命令执行', icon: 'terminal', run: () => { switchView('console'); emit('close') } },
  { title: '键空间分析', subtitle: '大 Key / 慢查询', icon: 'chart', run: () => { switchView('analysis'); emit('close') } },
  { title: '发布订阅', subtitle: '频道监听与发布', icon: 'radio', run: () => { switchView('pubsub'); emit('close') } },
  { title: '实例拓扑', subtitle: '哨兵 / 集群发现', icon: 'network', run: () => { switchView('topology'); emit('close') } },
  { title: '导入导出', subtitle: 'JSON / CSV', icon: 'download', run: () => { switchView('data'); emit('close') } },
  { title: '回收站', subtitle: '已删除键恢复', icon: 'trash', run: () => { switchView('recycle'); emit('close') } },
  { title: '审计日志', subtitle: '操作留痕查询', icon: 'shield', run: () => { switchView('audit'); emit('close') } },
  { title: '连接管理', subtitle: '实例登记与配置', icon: 'database', run: () => { switchView('connections'); emit('close') } },
  { title: '设置', subtitle: '主题 / 密度 / 采样', icon: 'settings', run: () => { switchView('settings'); emit('close') } },
]

// 键名搜索（防抖 SCAN）
let debounceTimer: number | undefined
watch(query, (q) => {
  clearTimeout(debounceTimer)
  keyResults.value = []
  if (!q || !currentConn()) return
  debounceTimer = window.setTimeout(async () => {
    try {
      const resp = await api.scanPage(store.currentConnId, store.currentDB, 0, `*${q}*`, '', 30)
      keyResults.value = resp.keys.map((k) => ({
        title: k.key,
        subtitle: `${k.type} · TTL ${k.ttl === -1 ? '永久' : Math.round(k.ttl / 1000) + 's'}`,
        icon: 'key',
        color: TYPE_META[k.type]?.color,
        run: () => {
          store.view = 'browser'
          window.dispatchEvent(new CustomEvent('redislens:open-key', { detail: k.key }))
          emit('close')
        },
      }))
    } catch (e) {
      toast('err', errMsg(e))
    }
  }, 250)
})

const q = computed(() => query.value.trim().toLowerCase())

const moduleResults = computed(() =>
  q.value ? modules.filter((m) => m.title.toLowerCase().includes(q.value) || m.subtitle.toLowerCase().includes(q.value)) : [],
)
const commandResults = computed(() =>
  q.value ? commands.filter((c) => c.title.toLowerCase().startsWith(q.value) || c.title.toLowerCase().includes(q.value)).slice(0, 8) : [],
)

const groups = computed(() => {
  const out: Array<{ label: string; items: Result[] }> = []
  if (moduleResults.value.length) out.push({ label: '功能模块', items: moduleResults.value })
  if (keyResults.value.length) out.push({ label: '键', items: keyResults.value })
  if (commandResults.value.length) out.push({ label: '命令', items: commandResults.value })
  return out
})

const flat = computed(() => groups.value.flatMap((g) => g.items))
function flatIndex(gi: number, ri: number): number {
  let idx = 0
  for (let i = 0; i < gi; i++) idx += groups.value[i].items.length
  return idx + ri
}

watch(() => props.open, async (v) => {
  if (v) {
    query.value = ''
    activeIdx.value = 0
    await nextTick()
    inputEl.value?.focus()
    loadCommands()
  }
})

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    emit('close')
  } else if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIdx.value = Math.min(activeIdx.value + 1, flat.value.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIdx.value = Math.max(activeIdx.value - 1, 0)
  } else if (e.key === 'Enter') {
    const r = flat.value[activeIdx.value]
    if (r) r.run()
  }
}
</script>

<style scoped>
.gs-mask {
  position: fixed;
  inset: 0;
  background: rgba(10, 12, 16, 0.4);
  z-index: 150;
  display: flex;
  justify-content: center;
  padding-top: 10vh;
}
.gs-panel {
  width: 560px;
  max-height: 60vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.gs-input-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}
.gs-input-row input {
  flex: 1;
  border: none;
  background: transparent;
  font-size: 14px;
  padding: 0;
}
.gs-input-row input:focus { box-shadow: none; }
.gs-input-row kbd {
  font-family: var(--font-mono);
  font-size: 10.5px;
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px 5px;
  color: var(--text-3);
}
.gs-results { overflow-y: auto; padding: 6px; }
.gs-group-label {
  font-size: 10.5px;
  font-weight: 700;
  color: var(--text-3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 8px 12px 4px;
}
.gs-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  text-align: left;
  font-size: 13px;
}
.gs-item.active { background: var(--bg-hover); }
.gs-main { font-family: var(--font-mono); font-size: 12.5px; color: var(--text); flex-shrink: 0; }
.gs-sub { font-size: 11.5px; margin-left: auto; max-width: 55%; }
.gs-hint { padding: 16px; font-size: 12.5px; text-align: center; }
</style>
