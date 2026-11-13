<template>
  <header class="topbar">
    <!-- 品牌 -->
    <div class="brand">
      <img :src="logoSvg" alt="RedisLens" class="brand-logo" />
    </div>

    <!-- 实例切换器 -->
    <div class="conn-switch" @click="switcherOpen = !switcherOpen">
      <span class="dot" :class="conn?.health || 'closed'"></span>
      <span class="conn-name ellipsis">{{ conn ? conn.name : '选择实例' }}</span>
      <Icon name="chevron-down" :size="14" class="faint" />
      <Transition name="pop">
        <div v-if="switcherOpen" class="switcher-panel card">
          <div v-for="c in grouped" :key="c.group" class="switcher-group">
            <div class="group-label">{{ c.group || '默认分组' }}</div>
            <button
              v-for="item in c.items"
              :key="item.id"
              class="switcher-item"
              :class="{ active: item.id === store.currentConnId }"
              @click.stop="pick(item)"
            >
              <span class="dot" :class="item.health"></span>
              <span class="ellipsis">{{ item.name }}</span>
              <span class="faint mono" style="font-size: 11px">{{ item.host }}:{{ item.port }}</span>
            </button>
          </div>
          <button class="switcher-item" @click.stop="openManage">
            <Icon name="settings" :size="14" />
            <span>管理连接…</span>
          </button>
        </div>
      </Transition>
    </div>

    <span class="grow"></span>

    <!-- 全局搜索 -->
    <button class="search-btn" @click="store.searchOpen = true">
      <Icon name="search" :size="14" />
      <span>搜索键名 / 命令…</span>
      <kbd>Ctrl K</kbd>
    </button>

    <span class="grow"></span>

    <!-- DB 切换 -->
    <div v-if="conn && conn.mode !== 'cluster'" class="db-switch">
      <span class="faint">DB</span>
      <select :value="store.currentDB" @change="changeDB($event)">
        <option v-for="i in 16" :key="i - 1" :value="i - 1">{{ i - 1 }}</option>
      </select>
    </div>
    <span v-else-if="conn" class="tag db-fixed">DB 0 · Cluster</span>

    <!-- 主题切换 -->
    <button class="icon-btn" :title="store.settings.theme === 'light' ? '切换深色' : '切换浅色'" @click="toggleTheme">
      <Icon :name="store.settings.theme === 'light' ? 'moon' : 'sun'" :size="16" />
    </button>
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { store, currentConn, openConnection } from '../lib/store'
import { refreshConnections } from '../lib/store'
import logoSvg from '../assets/logo.svg'
import type { ConnDTO } from '../lib/types'

const switcherOpen = ref(false)

const conn = computed(() => currentConn())

const grouped = computed(() => {
  const groups = new Map<string, ConnDTO[]>()
  for (const c of store.connections) {
    const g = c.group || '默认分组'
    if (!groups.has(g)) groups.set(g, [])
    groups.get(g)!.push(c)
  }
  return Array.from(groups.entries()).map(([group, items]) => ({ group, items }))
})

async function pick(c: ConnDTO) {
  switcherOpen.value = false
  if (c.open) {
    store.currentConnId = c.id
    if (store.view === 'welcome' || store.view === 'connections') store.view = 'browser'
  } else {
    await openConnection(c.id)
  }
}

function openManage() {
  switcherOpen.value = false
  store.view = 'connections'
}

function changeDB(e: Event) {
  store.currentDB = parseInt((e.target as HTMLSelectElement).value)
}

function toggleTheme() {
  store.settings.theme = store.settings.theme === 'light' ? 'dark' : 'light'
  import('../lib/store').then((m) => m.saveSettings({ theme: store.settings.theme }))
}
</script>

<style scoped>
.topbar {
  height: var(--topbar-h);
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 14px;
  flex-shrink: 0;
  z-index: 10;
}
.brand-logo { height: 28px; display: block; color: var(--text); }

.conn-switch {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  cursor: pointer;
  max-width: 260px;
  background: var(--bg);
  transition: border-color 0.15s;
}
.conn-switch:hover { border-color: var(--border-strong); }
.conn-name { font-weight: 600; font-size: 13px; }

.switcher-panel {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  width: 320px;
  max-height: 420px;
  overflow: auto;
  padding: 6px;
  z-index: 50;
}
.switcher-group { margin-bottom: 4px; }
.group-label {
  font-size: 11px;
  color: var(--text-3);
  font-weight: 600;
  padding: 6px 10px 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.switcher-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  text-align: left;
  font-size: 13px;
  color: var(--text);
}
.switcher-item:hover { background: var(--bg-hover); }
.switcher-item.active { background: var(--accent-soft); }

.search-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-3);
  background: var(--bg);
  font-size: 12.5px;
  min-width: 220px;
  transition: border-color 0.15s;
}
.search-btn:hover { border-color: var(--border-strong); }
.search-btn kbd {
  margin-left: auto;
  font-family: var(--font-mono);
  font-size: 10.5px;
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px 5px;
  background: var(--bg-hover);
}

.db-switch { display: flex; align-items: center; gap: 6px; font-size: 12.5px; }
.db-switch select { padding: 4px 8px; width: 64px; }
.db-fixed { background: var(--bg-hover); color: var(--text-2); }

.icon-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  color: var(--text-2);
  transition: background 0.15s;
}
.icon-btn:hover { background: var(--bg-hover); color: var(--text); }

.pop-enter-active, .pop-leave-active { transition: opacity 0.14s, transform 0.14s; }
.pop-enter-from, .pop-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
