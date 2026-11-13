<template>
  <div class="app-shell">
    <!-- 顶部全局栏 -->
    <TopBar />

    <!-- 全局搜索 -->
    <GlobalSearch :open="store.searchOpen" @close="store.searchOpen = false" />

    <div class="app-body">
      <!-- 左侧导航 -->
      <SideNav />

      <!-- 中央工作区 -->
      <main class="work-area">
        <component :is="viewComponent" :key="store.view + ':' + store.currentConnId" />

        <!-- 状态栏 -->
        <footer class="statusbar">
          <span v-if="conn" class="flex" style="gap: 6px">
            <span class="dot" :class="conn.health"></span>
            {{ conn.name }} · {{ connLabel }}
          </span>
          <span v-else class="faint">未连接</span>
          <span class="grow"></span>
          <span class="faint mono">RedisLens v1.0</span>
        </footer>
      </main>
    </div>

    <!-- Toast -->
    <Transition name="toast">
      <div v-if="store.toast" class="toast" :class="store.toast.kind">
        <Icon :name="store.toast.kind === 'err' ? 'alert' : store.toast.kind === 'ok' ? 'check' : 'info'" :size="15" />
        {{ store.toast.text }}
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'
import TopBar from './components/TopBar.vue'
import SideNav from './components/SideNav.vue'
import GlobalSearch from './components/GlobalSearch.vue'
import Icon from './components/Icon.vue'
import { store, initStore, currentConn, pushSample, refreshConnections } from './lib/store'
import { onEvent } from './lib/api'

import KeyBrowser from './views/KeyBrowser.vue'
import Dashboard from './views/Dashboard.vue'
import ConsoleView from './views/ConsoleView.vue'
import Analysis from './views/Analysis.vue'
import PubSubView from './views/PubSubView.vue'
import TopologyView from './views/TopologyView.vue'
import DataView from './views/DataView.vue'
import RecycleView from './views/RecycleView.vue'
import AuditView from './views/AuditView.vue'
import SettingsView from './views/SettingsView.vue'
import ConnManage from './views/ConnManage.vue'
import WelcomeView from './views/WelcomeView.vue'

const viewMap: Record<string, any> = {
  browser: KeyBrowser,
  dashboard: Dashboard,
  console: ConsoleView,
  analysis: Analysis,
  pubsub: PubSubView,
  topology: TopologyView,
  data: DataView,
  recycle: RecycleView,
  audit: AuditView,
  settings: SettingsView,
  connections: ConnManage,
  welcome: WelcomeView,
}

const viewComponent = computed(() => viewMap[store.view] || WelcomeView)
const conn = computed(() => currentConn())
const connLabel = computed(() => {
  const c = conn.value
  if (!c) return ''
  const mode = c.mode === 'standalone' ? '' : ` · ${c.mode.toUpperCase()}`
  const ro = c.readOnly ? ' · 只读' : ''
  return `${c.host}:${c.port} DB${c.mode === 'cluster' ? 0 : store.currentDB}${mode}${ro}`
})

// 事件监听
const offs: Array<() => void> = []
onMounted(async () => {
  await initStore()
  offs.push(
    onEvent('monitor:sample', (p: any) => {
      pushSample(p.connId, p.sample)
    }),
    onEvent('conn:health', (p: any) => {
      const c = store.connections.find((x) => x.id === p.connId)
      if (c) {
        c.health = p.status
        c.healthError = p.error || ''
      }
    }),
  )
  // 健康状态轮询刷新连接列表
  const timer = setInterval(refreshConnections, 15000)
  offs.push(() => clearInterval(timer))
})
onBeforeUnmount(() => offs.forEach((f) => f()))

// 全局快捷键：Ctrl+K 搜索
function onKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    store.searchOpen = !store.searchOpen
  }
}
window.addEventListener('keydown', onKeydown)
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<style>
.app-shell {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg);
}
.app-body {
  flex: 1;
  display: flex;
  min-height: 0;
}
.work-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}
.work-area > *:first-child { flex: 1; min-height: 0; }

.statusbar {
  height: 26px;
  border-top: 1px solid var(--border);
  background: var(--bg-panel);
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  font-size: 11.5px;
  color: var(--text-2);
  flex-shrink: 0;
}

.dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.dot.ok { background: var(--green); box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.15); }
.dot.down { background: var(--red); box-shadow: 0 0 0 3px rgba(220, 56, 45, 0.15); }
.dot.connecting { background: var(--amber); box-shadow: 0 0 0 3px rgba(217, 119, 6, 0.15); }
.dot.closed { background: var(--text-3); }

.toast {
  position: fixed;
  bottom: 44px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
  padding: 10px 18px;
  display: flex;
  align-items: center;
  gap: 8px;
  z-index: 300;
  font-size: 13px;
  max-width: 70vw;
}
.toast.ok svg { color: var(--green); }
.toast.err svg { color: var(--red); }
.toast.info svg { color: var(--t-hash); }
.toast-enter-active, .toast-leave-active { transition: opacity 0.2s, transform 0.2s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(8px); }
</style>
