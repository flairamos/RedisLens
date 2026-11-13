<template>
  <div class="keytree">
    <!-- 根节点 -->
    <button
      class="tree-node root"
      :class="{ active: selected.prefix === '' && !selected.key }"
      @click="selectRoot"
    >
      <Icon name="database" :size="13" />
      <span>全部键</span>
    </button>
    <div v-if="rootNodes.length || loaded">
      <TreeItem v-for="n in rootNodes" :key="n.prefix" :node="n" :depth="0" />
    </div>
    <div v-else class="tree-hint faint">
      <button class="tree-node" @click="loadRoot">
        <Icon name="chevron-right" :size="11" />
        <span>加载键空间…</span>
      </button>
    </div>
    <button class="tree-node refresh" @click="reload" title="刷新键空间树">
      <Icon name="refresh" :size="12" />
      <span style="font-size: 12px">刷新树</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import Icon from './Icon.vue'
import TreeItem from './TreeItem.vue'
import { store, currentConn, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import type { TreeNode } from '../lib/types'

const conn = computed(() => currentConn())
const rootNodes = ref<TreeNode[]>([])
const loaded = ref(false)
// 与 TreeItem 共享同一 reactive 树状态
const selected = ((window as any).__rlTree ||= reactive({
  expanded: {} as Record<string, boolean>,
  children: {} as Record<string, unknown[]>,
  selected: { prefix: '', key: '' },
})).selected

async function loadRoot() {
  if (!conn.value) return
  try {
    rootNodes.value = await api.keyTree(conn.value.id, store.currentDB, '')
    loaded.value = true
  } catch (e) {
    toast('err', errMsg(e))
  }
}

// 清空缓存并重载
function reload() {
  const ts = (window as any).__rlTree
  if (ts) {
    ts.expanded = {}
    ts.children = {}
  }
  selected.prefix = ''
  selected.key = ''
  loadRoot()
}

function selectRoot() {
  selected.prefix = ''
  selected.key = ''
  window.dispatchEvent(new CustomEvent('redislens:select-prefix', { detail: '' }))
}

// DB 或连接变化时重置
watch(() => [conn.value?.id, store.currentDB], () => {
  const ts = (window as any).__rlTree
  if (ts) {
    ts.expanded = {}
    ts.children = {}
  }
  loaded.value = false
  rootNodes.value = []
}, { immediate: true })

// 首次进入自动加载
watch(loaded, (v) => { if (!v) loadRoot() }, { immediate: true })

defineExpose({ loadRoot, reload })
</script>

<style scoped>
.keytree {
  flex: 1;
  overflow-y: auto;
  padding: 2px 8px 12px;
  min-height: 0;
}
.tree-node {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 5px 10px;
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  color: var(--text-2);
  text-align: left;
}
.tree-node:hover { background: var(--bg-hover); color: var(--text); }
.tree-node.root {
  font-weight: 600;
  color: var(--text);
  margin-bottom: 2px;
}
.tree-node.root.active { background: var(--accent-soft); color: var(--red); }
.tree-hint { padding: 0; }
.refresh { color: var(--text-3); margin-top: 4px; }
</style>

<style scoped>
.keytree {
  flex: 1;
  overflow-y: auto;
  padding: 4px 8px 12px;
  min-height: 0;
}
.tree-node {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 5px 10px;
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  color: var(--text-2);
  text-align: left;
}
.tree-node:hover { background: var(--bg-hover); color: var(--text); }
.tree-node.active { background: var(--accent-soft); color: var(--red); font-weight: 600; }
.tree-node.root { font-weight: 600; color: var(--text); margin-bottom: 2px; }
.tree-loading {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 10px 5px 22px;
  font-size: 12.5px;
  color: var(--text-3);
  cursor: pointer;
}
.tree-loading:hover { color: var(--text); }
</style>
