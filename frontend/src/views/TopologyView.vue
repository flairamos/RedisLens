<template>
  <div class="topo">
    <div class="tp-head">
      <h2>实例拓扑</h2>
      <span class="grow"></span>
      <button class="btn" @click="load" :disabled="loading">
        <span v-if="loading" class="spinner"></span>
        <Icon v-else name="refresh" :size="14" />
        {{ loading ? '发现中…' : '重新发现' }}
      </button>
    </div>

    <template v-if="topo">
      <div class="tp-summary card">
        <div class="tp-item">
          <span class="faint">部署模式</span>
          <span class="tp-mode">{{ modeLabel }}</span>
        </div>
        <div class="tp-item" v-if="topo.master">
          <span class="faint">主节点</span>
          <span class="mono">{{ topo.master }}</span>
        </div>
        <div class="tp-item" v-if="topo.mode === 'cluster'">
          <span class="faint">分片数</span>
          <span class="mono">{{ topo.shards }}</span>
        </div>
        <div class="tp-item">
          <span class="faint">节点数</span>
          <span class="mono">{{ topo.nodes.length }}</span>
        </div>
      </div>

      <!-- 集群分片视图 -->
      <div v-if="topo.mode === 'cluster'" class="shards">
        <div v-for="(shard, i) in shards" :key="i" class="card shard">
          <div class="shard-head">
            <Icon name="server" :size="15" />
            <b>分片 {{ i + 1 }}</b>
            <span class="tag slots-tag">{{ shard.slots }}</span>
          </div>
          <div class="shard-nodes">
            <div v-for="n in shard.nodes" :key="n.addr + n.role" class="node-row">
              <span class="node-role" :class="n.role">{{ n.role === 'master' ? '主' : '从' }}</span>
              <span class="mono">{{ n.addr }}</span>
              <span v-if="n.id" class="faint mono" style="font-size: 10.5px">{{ n.id.slice(0, 12) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 主从列表视图 -->
      <div v-else class="card">
        <table class="tbl">
          <thead><tr><th style="width: 90px">角色</th><th>地址</th><th v-if="topo.mode === 'sentinel'" style="width: 90px">类型</th><th style="width: 110px">状态</th></tr></thead>
          <tbody>
            <tr v-for="(n, i) in topo.nodes" :key="n.addr + i">
              <td>
                <span class="node-role" :class="n.role">
                  {{ n.role === 'master' ? '主节点' : n.role === 'sentinel' ? '哨兵' : '副本' }}
                </span>
              </td>
              <td class="mono">{{ n.addr }}</td>
              <td v-if="topo.mode === 'sentinel'" class="faint">{{ n.role === 'sentinel' ? 'Sentinel' : 'Redis' }}</td>
              <td>
                <span :style="{ color: n.state && n.state !== 'online' ? 'var(--red)' : 'var(--green)' }">
                  {{ n.state || 'online' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <div v-else class="empty" style="flex: 1">
      <Icon name="network" :size="44" />
      <div>{{ loading ? '正在自动发现拓扑…' : '点击「重新发现」查看主从 / 分片拓扑' }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import { currentConn, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import type { NodeInfo, Topology } from '../lib/types'

const conn = computed(() => currentConn())
const topo = ref<Topology | null>(null)
const loading = ref(false)

const modeLabel = computed(() =>
  ({ standalone: '单机', sentinel: '哨兵（Sentinel）', cluster: '集群（Cluster）' }[topo.value?.mode || ''] || topo.value?.mode),
)

// 集群分片聚合
const shards = computed(() => {
  if (!topo.value || topo.value.mode !== 'cluster') return []
  const bySlot = new Map<string, NodeInfo[]>()
  for (const n of topo.value.nodes) {
    const key = n.slots || ''
    if (!bySlot.has(key)) bySlot.set(key, [])
    bySlot.get(key)!.push(n)
  }
  return Array.from(bySlot.entries())
    .sort((a, b) => parseInt(a[0]) - parseInt(b[0]))
    .map(([slots, nodes]) => ({ slots, nodes }))
})

async function load() {
  const c = conn.value
  if (!c) return
  loading.value = true
  try {
    topo.value = await api.getTopology(c.id)
  } catch (e) {
    toast('err', errMsg(e))
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => conn.value?.id, () => {
  topo.value = null
  load()
})
</script>

<style scoped>
.topo {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.tp-head { display: flex; align-items: center; gap: 12px; }
.tp-head h2 { margin: 0; font-size: 17px; font-weight: 700; }

.tp-summary { display: flex; gap: 40px; padding: 16px 20px; }
.tp-item { display: flex; flex-direction: column; gap: 4px; }
.tp-item > span:last-child { font-size: 15px; font-weight: 600; }
.tp-mode { color: var(--red); font-weight: 700; }

.shards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; }
.shard { padding: 14px 16px; }
.shard-head { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.slots-tag { background: var(--accent-soft); color: var(--red); font-family: var(--font-mono); }
.shard-nodes { display: flex; flex-direction: column; gap: 8px; }
.node-row { display: flex; align-items: center; gap: 10px; font-size: 12.5px; }
.node-role {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  flex-shrink: 0;
}
.node-role.master { background: rgba(220, 56, 45, 0.1); color: var(--red); }
.node-role.replica { background: rgba(59, 130, 246, 0.1); color: var(--t-hash); }
.node-role.sentinel { background: rgba(6, 182, 212, 0.1); color: var(--t-stream); }
</style>
