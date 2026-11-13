<template>
  <div class="tree-item">
    <!-- 文件夹 -->
    <div v-if="node.isFolder">
      <button
        class="tree-node"
        :class="{ active: selectedPrefix === node.prefix }"
        :style="{ paddingLeft: 10 + depth * 16 + 'px' }"
        @click="toggle"
      >
        <span class="chev" :class="{ open: expanded }">
          <Icon name="chevron-right" :size="11" />
        </span>
        <Icon name="folder" :size="13" />
        <span class="ellipsis">{{ node.name }}</span>
        <span v-if="children.length" class="faint count">{{ children.length }}</span>
      </button>
      <div v-if="expanded" class="tree-children">
        <div v-if="loading" class="tree-loading"><span class="spinner" style="width: 12px; height: 12px"></span></div>
        <div v-else-if="children.length === 0" class="tree-empty faint">（空）</div>
        <TreeItem
          v-for="c in children"
          :key="c.prefix"
          :node="c"
          :depth="depth + 1"
        />
      </div>
    </div>
    <!-- 键 -->
    <button
      v-else
      class="tree-node leaf"
      :class="{ active: selectedKey === node.prefix }"
      :style="{ paddingLeft: 10 + depth * 16 + 18 + 'px' }"
      @click="selectKey"
      :title="node.prefix"
    >
      <span class="type-dot" :style="{ background: typeMeta(node.type || 'none').color }"></span>
      <span class="ellipsis">{{ node.name }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import Icon from './Icon.vue'
import { store, currentConn } from '../lib/store'
import { api } from '../lib/api'
import { typeMeta } from '../lib/format'
import type { TreeNode } from '../lib/types'

const props = defineProps<{ node: TreeNode; depth: number }>()

// 跨组件树状态（模块级 reactive，视图切换不丢失）
const treeStore = ((window as any).__rlTree ||= reactive({
  expanded: {} as Record<string, boolean>,
  children: {} as Record<string, TreeNode[]>,
  selected: { prefix: '', key: '' },
}))
const expandedMap = treeStore.expanded
const childrenMap = treeStore.children
const selected = treeStore.selected

const expanded = computed(() => !!expandedMap[props.node.prefix])
const children = computed<TreeNode[]>(() => childrenMap[props.node.prefix] || [])
const loading = ref(false)
const selectedPrefix = computed(() => selected.prefix)
const selectedKey = computed(() => selected.key)

async function toggle() {
  if (expanded.value) {
    expandedMap[props.node.prefix] = false
    return
  }
  expandedMap[props.node.prefix] = true
  if (!childrenMap[props.node.prefix]) {
    loading.value = true
    try {
      childrenMap[props.node.prefix] = await api.keyTree(currentConn()!.id, store.currentDB, props.node.prefix)
    } finally {
      loading.value = false
    }
  }
  // 同步键列表选择
  selected.prefix = props.node.prefix
  window.dispatchEvent(new CustomEvent('redislens:select-prefix', { detail: props.node.prefix }))
}

function selectKey() {
  selected.key = props.node.prefix
  window.dispatchEvent(new CustomEvent('redislens:select-key', { detail: props.node.prefix }))
}
</script>

<style scoped>
.tree-node {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  color: var(--text-2);
  text-align: left;
  font-family: var(--font-mono);
  min-height: 26px;
}
.tree-node:hover { background: var(--bg-hover); color: var(--text); }
.tree-node.active { background: var(--accent-soft); color: var(--red); }
.chev { display: inline-flex; transition: transform 0.15s; color: var(--text-3); }
.chev.open { transform: rotate(90deg); }
.count { font-size: 10.5px; margin-left: auto; font-family: var(--font-mono); }
.type-dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
  flex-shrink: 0;
}
.tree-children { position: relative; }
.tree-loading { padding: 6px 10px 6px 30px; }
.tree-empty { padding: 4px 10px 4px 30px; font-size: 11.5px; }
</style>
