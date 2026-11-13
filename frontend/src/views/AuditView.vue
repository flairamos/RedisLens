<template>
  <div class="audit">
    <div class="au-head">
      <h2>审计日志</h2>
      <span class="faint" style="font-size: 12.5px">本地 JSONL 留存 ≥ 90 天 · 危险命令强制记录</span>
      <span class="grow"></span>
      <select v-model="levelFilter" style="width: 120px">
        <option value="">全部级别</option>
        <option value="danger">危险</option>
        <option value="warn">警告</option>
        <option value="info">信息</option>
      </select>
      <select v-model="catFilter" style="width: 130px">
        <option value="">全部类别</option>
        <option v-for="c in categories" :key="c" :value="c">{{ catLabel(c) }}</option>
      </select>
      <button class="btn" @click="load"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <div class="card" style="overflow: auto; flex: 1">
      <table class="tbl">
        <thead>
          <tr>
            <th style="width: 160px">时间</th>
            <th style="width: 110px">实例</th>
            <th style="width: 90px">类别</th>
            <th style="width: 120px">动作</th>
            <th>对象</th>
            <th style="width: 90px">结果</th>
            <th style="width: 200px">详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in filtered" :key="i">
            <td class="mono faint">{{ new Date(e.time).toLocaleString('zh-CN', { hour12: false }) }}</td>
            <td class="ellipsis">{{ e.connName || e.connId }}</td>
            <td><span class="tag" :style="catStyle(e.category)">{{ catLabel(e.category) }}</span></td>
            <td class="mono" :style="{ color: e.level === 'danger' ? 'var(--red)' : e.level === 'warn' ? 'var(--amber)' : 'var(--text)' }">{{ e.action }}</td>
            <td class="mono ellipsis" :title="e.target">{{ e.target }}</td>
            <td>
              <span :style="{ color: e.result === 'ok' ? 'var(--green)' : e.result === 'blocked' ? 'var(--amber)' : 'var(--red)' }">
                {{ e.result === 'ok' ? '成功' : e.result === 'blocked' ? '拦截' : '失败' }}
              </span>
            </td>
            <td class="mono faint ellipsis" :title="e.detail">{{ e.detail }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="!filtered.length" class="empty" style="padding: 40px">
        <Icon name="shield" :size="40" />
        <div>暂无审计记录</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import type { AuditEntry } from '../lib/types'

const entries = ref<AuditEntry[]>([])
const levelFilter = ref('')
const catFilter = ref('')

const categories = ['connection', 'command', 'dangerous', 'key_edit', 'key_delete', 'export', 'import', 'analysis']

function catLabel(c: string): string {
  return (
    {
      connection: '连接',
      command: '命令',
      dangerous: '危险命令',
      key_edit: '键编辑',
      key_delete: '键删除',
      export: '导出',
      import: '导入',
      analysis: '分析',
    }[c] || c
  )
}

function catStyle(c: string): any {
  const map: Record<string, string> = {
    connection: 'var(--t-hash)',
    command: 'var(--text-2)',
    dangerous: 'var(--red)',
    key_edit: 'var(--t-list)',
    key_delete: 'var(--red)',
    export: 'var(--t-stream)',
    import: 'var(--t-set)',
    analysis: 'var(--t-zset)',
  }
  const color = map[c] || 'var(--text-3)'
  return { background: `color-mix(in srgb, ${color} 12%, transparent)`, color }
}

const filtered = computed(() =>
  entries.value.filter(
    (e) =>
      (!levelFilter.value || e.level === levelFilter.value) &&
      (!catFilter.value || e.category === catFilter.value),
  ),
)

async function load() {
  try {
    entries.value = await api.queryAudit(500)
  } catch (e) {
    toast('err', errMsg(e))
  }
}

onMounted(load)
</script>

<style scoped>
.audit {
  padding: 16px 20px;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow: hidden;
}
.au-head { display: flex; align-items: center; gap: 10px; }
.au-head h2 { margin: 0; font-size: 17px; font-weight: 700; }
</style>
