<template>
  <nav class="sidenav" :class="{ collapsed: store.sidenavCollapsed }">
    <!-- 模块导航 -->
    <div class="nav-section">
      <button
        v-for="item in navItems"
        :key="item.view"
        class="nav-item"
        :class="{ active: store.view === item.view }"
        :title="item.label"
        @click="switchView(item.view)"
      >
        <Icon :name="item.icon" :size="16" />
        <span v-if="!store.sidenavCollapsed" class="ellipsis">{{ item.label }}</span>
      </button>
    </div>

    <div class="nav-divider"></div>

    <!-- 键空间树 -->
    <div v-if="conn && store.view !== 'connections'" class="tree-wrap">
      <div class="tree-head" v-if="!store.sidenavCollapsed">
        <span class="tree-title">
          <Icon name="layers" :size="13" />
          键空间树
        </span>
        <span class="faint mono" style="font-size: 11px">DB{{ store.currentDB }}</span>
      </div>
      <KeyTree v-if="!store.sidenavCollapsed" />
    </div>

    <span class="grow"></span>

    <!-- 折叠按钮 -->
    <button class="collapse-btn" @click="store.sidenavCollapsed = !store.sidenavCollapsed">
      <Icon :name="store.sidenavCollapsed ? 'chevron-right' : 'chevron-left'" :size="14" />
    </button>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'
import KeyTree from './KeyTree.vue'
import { store, currentConn, switchView } from '../lib/store'

const conn = computed(() => currentConn())

const navItems = [
  { view: 'browser', icon: 'key', label: '数据浏览' },
  { view: 'dashboard', icon: 'activity', label: '监控仪表盘' },
  { view: 'console', icon: 'terminal', label: '命令行控制台' },
  { view: 'analysis', icon: 'chart', label: '键空间分析' },
  { view: 'pubsub', icon: 'radio', label: '发布订阅' },
  { view: 'topology', icon: 'network', label: '实例拓扑' },
  { view: 'data', icon: 'download', label: '导入导出' },
  { view: 'recycle', icon: 'trash', label: '回收站' },
  { view: 'audit', icon: 'shield', label: '审计日志' },
  { view: 'connections', icon: 'database', label: '连接管理' },
  { view: 'settings', icon: 'settings', label: '设置' },
] as const
</script>

<style scoped>
.sidenav {
  width: var(--sidenav-w);
  background: var(--bg-panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  transition: width 0.18s ease;
  overflow: hidden;
}
.sidenav.collapsed { width: 52px; }

.nav-section {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  color: var(--text-2);
  font-size: 13px;
  text-align: left;
  transition: background 0.12s, color 0.12s;
  white-space: nowrap;
}
.nav-item:hover { background: var(--bg-hover); color: var(--text); }
.nav-item.active {
  background: var(--accent-soft);
  color: var(--red);
  font-weight: 600;
}
.sidenav.collapsed .nav-item { justify-content: center; padding: 8px; }

.nav-divider { height: 1px; background: var(--border); margin: 4px 12px; }

.tree-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--border);
}
.tree-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px 6px;
}
.tree-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--text-3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.collapse-btn {
  margin: 8px;
  padding: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  color: var(--text-3);
}
.collapse-btn:hover { background: var(--bg-hover); color: var(--text); }
</style>
