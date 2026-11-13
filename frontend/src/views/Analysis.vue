<template>
  <div class="analysis">
    <div class="an-head">
      <h2>键空间分析</h2>
      <span class="grow"></span>
      <button v-if="!scanning" class="btn primary" @click="startScan">
        <Icon name="zap" :size="15" /> 开始大 Key 扫描
      </button>
      <button v-else class="btn danger-ghost" @click="stopScan">
        <Icon name="stop" :size="14" /> 停止
      </button>
    </div>

    <!-- 扫描进度 -->
    <div v-if="scanning || lastProgress" class="card progress-card">
      <div class="progress-row">
        <span class="spinner" v-if="scanning"></span>
        <span>{{ scanning ? '正在扫描键空间…' : `上次扫描：${report ? report.scannedKeys.toLocaleString() : '-'} 键 / ${report?.durationMs ?? '-'}ms` }}</span>
        <span class="grow"></span>
        <span class="mono faint">{{ progress.scanned.toLocaleString() }} / {{ progress.total.toLocaleString() }}</span>
      </div>
      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: progressPct + '%' }"></div>
      </div>
    </div>

    <div class="an-grid" v-if="report">
      <!-- 内存 Top -->
      <div class="card an-card">
        <h4><Icon name="layers" :size="14" /> 大 Key 排行（内存）</h4>
        <table class="tbl">
          <thead><tr><th>键</th><th>类型</th><th style="text-align:right">内存</th><th style="text-align:right">元素</th></tr></thead>
          <tbody>
            <tr v-for="it in report.topByMemory" :key="'m' + it.key" class="clickable" @click="openKey(it.key)">
              <td class="mono ellipsis" :title="it.key">{{ it.key }}</td>
              <td><TypeBadge :type="it.type" /></td>
              <td class="mono" style="text-align: right">{{ fmtBytes(it.bytes) }}</td>
              <td class="mono faint" style="text-align: right">{{ fmtNum(it.elements) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 元素数 Top -->
      <div class="card an-card">
        <h4><Icon name="list" :size="14" /> 大 Key 排行（元素数）</h4>
        <table class="tbl">
          <thead><tr><th>键</th><th>类型</th><th style="text-align:right">元素</th><th style="text-align:right">内存</th></tr></thead>
          <tbody>
            <tr v-for="it in report.topByElement" :key="'e' + it.key" class="clickable" @click="openKey(it.key)">
              <td class="mono ellipsis" :title="it.key">{{ it.key }}</td>
              <td><TypeBadge :type="it.type" /></td>
              <td class="mono" style="text-align: right">{{ fmtNum(it.elements) }}</td>
              <td class="mono faint" style="text-align: right">{{ fmtBytes(it.bytes) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 类型分布 -->
      <div class="card an-card">
        <h4><Icon name="chart" :size="14" /> 类型分布</h4>
        <div class="chart" ref="pieChart"></div>
      </div>

      <!-- TTL 分布 -->
      <div class="card an-card">
        <h4><Icon name="clock" :size="14" /> TTL 分布</h4>
        <div class="chart" ref="ttlChart"></div>
      </div>
    </div>
    <div v-else class="empty" style="flex:1">
      <Icon name="zap" :size="44" />
      <div>扫描全库找出内存与元素数最高的键，分析类型与 TTL 分布</div>
      <div class="faint" style="font-size: 12px">SCAN 渐进扫描 · 不阻塞生产实例 · 可随时停止</div>
    </div>

    <!-- 慢查询与热命令 -->
    <div class="an-grid2">
      <div class="card an-card">
        <div class="card-head">
          <h4><Icon name="clock" :size="14" /> 慢查询日志</h4>
          <button class="btn sm" @click="loadSlowLog"><Icon name="refresh" :size="13" /> 刷新</button>
        </div>
        <table class="tbl" v-if="slowLogs.length">
          <thead><tr><th style="width:130px">时间</th><th style="width:90px">耗时</th><th>命令</th><th style="width:130px">客户端</th></tr></thead>
          <tbody>
            <tr v-for="l in slowLogs" :key="l.id" class="clickable" @click="openSlowKey(l)">
              <td class="mono faint">{{ fmtTime(l.time) }}</td>
              <td class="mono" :style="{ color: l.duration > 10000 ? 'var(--red)' : 'var(--amber)' }">{{ fmtUsec(l.duration) }}</td>
              <td class="mono ellipsis" :title="l.command">{{ l.command }}</td>
              <td class="mono faint">{{ l.client }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty" style="padding: 20px"><span class="faint">无慢查询记录</span></div>
      </div>

      <div class="card an-card">
        <div class="card-head">
          <h4><Icon name="zap" :size="14" /> 热命令统计（commandstats）</h4>
          <button class="btn sm" @click="loadCmdStats"><Icon name="refresh" :size="13" /> 刷新</button>
        </div>
        <table class="tbl" v-if="cmdStats.length">
          <thead><tr><th>命令</th><th style="text-align:right">调用次数</th><th style="text-align:right">平均耗时</th></tr></thead>
          <tbody>
            <tr v-for="c in cmdStats.slice(0, 15)" :key="c.command">
              <td class="mono" style="color: var(--t-hash)">{{ c.command }}</td>
              <td class="mono" style="text-align: right">{{ fmtNum(c.calls) }}</td>
              <td class="mono faint" style="text-align: right">{{ c.calls > 0 ? fmtUsec(Math.round(c.usec / c.calls)) : '-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty" style="padding: 20px"><span class="faint">暂无统计</span></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { PieChart, BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import Icon from '../components/Icon.vue'
import TypeBadge from '../components/TypeBadge.vue'
import { store, currentConn, toast } from '../lib/store'
import { api, errMsg, onEvent } from '../lib/api'
import { fmtBytes, fmtNum, fmtTime, fmtUsec, TYPE_META } from '../lib/format'
import type { BigKeyReport, CmdStat, SlowLogEntry } from '../lib/types'

echarts.use([PieChart, BarChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const conn = computed(() => currentConn())
const scanning = ref(false)
const report = ref<BigKeyReport | null>(null)
const progress = reactive({ scanned: 0, total: 0 })
const lastProgress = ref(false)
const taskId = ref('')
const slowLogs = ref<SlowLogEntry[]>([])
const cmdStats = ref<CmdStat[]>([])

const progressPct = computed(() =>
  progress.total > 0 ? Math.min(100, Math.round((progress.scanned / progress.total) * 100)) : 0,
)

async function startScan() {
  const c = conn.value
  if (!c) return
  scanning.value = true
  lastProgress.value = true
  progress.scanned = 0
  progress.total = 0
  try {
    taskId.value = await api.beginBigKeyScan(c.id, 20, 2000000)
  } catch (e) {
    toast('err', errMsg(e))
    scanning.value = false
  }
}

function stopScan() {
  if (taskId.value) api.stopBigKeyScan(taskId.value).catch(() => {})
}

// 图表
const pieChart = ref<HTMLElement>()
const ttlChart = ref<HTMLElement>()
let pieIns: echarts.ECharts | null = null
let ttlIns: echarts.ECharts | null = null

function renderPie() {
  if (!pieChart.value || !report.value) return
  pieIns ||= echarts.init(pieChart.value)
  const data = Object.entries(report.value.typeDist)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([k, v]) => ({
      name: k,
      value: v,
      itemStyle: { color: TYPE_META[k]?.color || 'var(--t-none)' },
    }))
  pieIns.setOption({
    animation: false,
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        label: { color: '#5b6472', fontSize: 11 },
        data,
      },
    ],
  })
}

function renderTTL() {
  if (!ttlChart.value || !report.value) return
  ttlIns ||= echarts.init(ttlChart.value)
  const order = ['永不过期', '≤1分钟', '≤1小时', '≤1天', '≤7天', '≤30天', '>30天', 'error']
  const keys = Object.keys(report.value.ttlBuckets).sort((a, b) => order.indexOf(a) - order.indexOf(b))
  ttlIns.setOption({
    animation: false,
    grid: { left: 80, right: 20, top: 10, bottom: 30 },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'value', axisLabel: { color: '#5b6472', fontSize: 10.5 } },
    yAxis: {
      type: 'category',
      data: keys,
      axisLabel: { color: '#5b6472', fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: keys.map((k) => report.value!.ttlBuckets[k]),
        itemStyle: { color: '#3b82f6', borderRadius: [0, 4, 4, 0] },
        label: { show: true, position: 'right', color: '#5b6472', fontSize: 10.5 },
      },
    ],
  })
}

function openKey(key: string) {
  store.view = 'browser'
  setTimeout(() => window.dispatchEvent(new CustomEvent('redislens:open-key', { detail: key })), 120)
}

function openSlowKey(l: SlowLogEntry) {
  // 尝试提取命令中的第一个键参数
  const parts = l.command.split(/\s+/)
  if (parts.length > 1) openKey(parts[1].replace(/^["']|["']$/g, ''))
}

async function loadSlowLog() {
  const c = conn.value
  if (!c) return
  try {
    slowLogs.value = await api.getSlowLog(c.id, 100)
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function loadCmdStats() {
  const c = conn.value
  if (!c) return
  try {
    cmdStats.value = await api.getCommandStats(c.id)
  } catch (e) {
    toast('err', errMsg(e))
  }
}

// 事件
let offP: (() => void) | null = null
let offD: (() => void) | null = null
onMounted(() => {
  offP = onEvent('analysis:progress', (p: any) => {
    progress.scanned = p.scanned
    progress.total = p.totalKeys
  }) as any
  offD = onEvent('analysis:done', (p: any) => {
    if (p.connId !== conn.value?.id) return
    report.value = p.report
    scanning.value = false
    renderPie()
    renderTTL()
    if (!p.report.stopped) toast('ok', `扫描完成：${p.report.scannedKeys.toLocaleString()} 键，耗时 ${p.report.durationMs}ms`)
  }) as any
  loadSlowLog()
  loadCmdStats()
})
onBeforeUnmount(() => {
  offP?.()
  offD?.()
  pieIns?.dispose()
  ttlIns?.dispose()
})

watch(() => conn.value?.id, () => {
  report.value = null
  scanning.value = false
  slowLogs.value = []
  cmdStats.value = []
  loadSlowLog()
  loadCmdStats()
})
</script>

<style scoped>
.analysis {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.an-head { display: flex; align-items: center; gap: 12px; }
.an-head h2 { margin: 0; font-size: 17px; font-weight: 700; }

.progress-card { padding: 14px 16px; }
.progress-row { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; font-size: 13px; }
.progress-bar {
  height: 6px;
  background: var(--bg-hover);
  border-radius: 3px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: var(--red);
  border-radius: 3px;
  transition: width 0.3s;
}

.an-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.an-grid2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.an-card { padding: 14px; min-width: 0; }
.an-card h4 {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--text-2);
  display: flex;
  align-items: center;
  gap: 6px;
}
.card-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.card-head h4 { margin: 0; }
.chart { height: 200px; }
.tbl { max-height: 260px; overflow-y: auto; display: block; }
.tbl thead { display: table; width: 100%; }
.tbl tbody { display: table; width: 100%; }
</style>
