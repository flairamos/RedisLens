// 全局响应式状态（轻量 store，无需 Pinia）
import { reactive, watch } from 'vue'
import { api } from './api'
import type { ConnDTO, Sample, Settings } from './types'

export type ViewName =
  | 'browser' | 'dashboard' | 'console' | 'analysis' | 'pubsub'
  | 'topology' | 'data' | 'recycle' | 'audit' | 'settings' | 'connections' | 'welcome'

interface Store {
  settings: Settings
  connections: ConnDTO[]
  currentConnId: string
  currentDB: number
  view: ViewName
  sidenavCollapsed: boolean
  detailVisible: boolean
  searchOpen: boolean
  toast: { kind: 'ok' | 'err' | 'info'; text: string; at: number } | null
  connSamples: Record<string, Sample[]> // 最近一帧监控（供顶栏迷你展示）
}

export const store = reactive<Store>({
  settings: {
    theme: 'light',
    density: 'comfortable',
    monitorIntervalSec: 10,
    confirmDanger: true,
    logEnabled: true,
    scanCount: 500,
    valuePreviewLimit: 2 << 20,
  },
  connections: [],
  currentConnId: '',
  currentDB: 0,
  view: 'welcome',
  sidenavCollapsed: false,
  detailVisible: true,
  searchOpen: false,
  toast: null,
  connSamples: {},
})

export const currentConn = () => store.connections.find((c) => c.id === store.currentConnId)

// 主题与密度应用到根节点
export function applyTheme() {
  document.documentElement.dataset.theme = store.settings.theme
  document.documentElement.dataset.density = store.settings.density
}

export async function loadSettings() {
  store.settings = await api.getSettings()
  applyTheme()
}

export async function saveSettings(patch: Partial<Settings>) {
  Object.assign(store.settings, patch)
  applyTheme()
  await api.updateSettings({ ...store.settings })
}

export async function refreshConnections() {
  store.connections = await api.listConnections()
}

// Toast 轻提示
export function toast(kind: 'ok' | 'err' | 'info', text: string) {
  store.toast = { kind, text, at: Date.now() }
  setTimeout(() => {
    if (store.toast && Date.now() - store.toast.at >= 2800) store.toast = null
  }, 3000)
}

// 打开连接并切换到键浏览
export async function openConnection(id: string) {
  const conn = store.connections.find((c) => c.id === id)
  if (!conn) return
  try {
    await api.openConnection(id)
    store.currentConnId = id
    store.currentDB = conn.db || 0
    store.view = 'browser'
    await refreshConnections()
    toast('ok', `已连接 ${conn.name}`)
  } catch (e: any) {
    toast('err', `连接失败：${e?.message || e}`)
    await refreshConnections()
    throw e
  }
}

// 切换视图（需已连接）
export function switchView(v: ViewName) {
  if (v !== 'connections' && v !== 'settings' && v !== 'welcome' && v !== 'audit' && !store.currentConnId) {
    toast('info', '请先连接一个实例')
    return
  }
  store.view = v
}

// 监控事件驱动（最新一帧）
export function pushSample(connId: string, sample: Sample) {
  const arr = store.connSamples[connId] || []
  arr.push(sample)
  if (arr.length > 4320) arr.splice(0, arr.length - 4320) // ~12h@10s
  store.connSamples[connId] = arr
}

export function samplesOf(connId: string): Sample[] {
  return store.connSamples[connId] || []
}

// 初始化
export async function initStore() {
  await loadSettings()
  await refreshConnections()
  watch(() => store.settings.theme, applyTheme)
  watch(() => store.settings.density, applyTheme)
}
