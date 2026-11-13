<template>
  <div class="dash">
    <!-- 头部 -->
    <div class="dash-head">
      <h2>监控仪表盘</h2>
      <span class="faint" v-if="info">Redis {{ info.version }} · {{ info.role }} · {{ intervalSec }}s 采样</span>
      <span class="grow"></span>
      <div class="window-picker">
        <button
          v-for="w in windows"
          :key="w.sec"
          class="btn sm"
          :class="{ primary: windowSec === w.sec }"
          @click="setWindow(w.sec)"
        >{{ w.label }}</button>
      </div>
      <label class="live-toggle">
        <input type="checkbox" v-model="live" />
        实时
      </label>
    </div>

    <!-- 指标卡片 -->
    <div class="metric-cards">
      <div class="metric card" v-for="m in cards" :key="m.label">
        <div class="m-label">{{ m.label }}</div>
        <div class="m-value mono" :style="{ color: m.color }">{{ m.value }}</div>
        <div class="m-sub faint">{{ m.sub }}</div>
      </div>
    </div>

    <!-- 图表区 -->
    <div class="charts">
      <div class="chart-card card">
        <h4>QPS 与命中率</h4>
        <div class="chart" ref="qpsChart"></div>
      </div>
      <div class="chart-card card">
        <h4>内存使用与碎片率</h4>
        <div class="chart" ref="memChart"></div>
      </div>
      <div class="chart-card card">
        <h4>网络 IO</h4>
        <div class="chart" ref="netChart"></div>
      </div>
      <div class="chart-card card">
        <h4>连接数</h4>
        <div class="chart" ref="connChart"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { store, currentConn, samplesOf } from '../lib/store'
import { api } from '../lib/api'
import { fmtBytes, fmtNum } from '../lib/format'
import type { Sample } from '../lib/types'

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, CanvasRenderer])

const conn = computed(() => currentConn())
const windows = [
  { label: '10分钟', sec: 600 },
  { label: '1小时', sec: 3600 },
  { label: '6小时', sec: 21600 },
  { label: '24小时', sec: 86400 },
]
const windowSec = ref(3600)
const live = ref(true)
const intervalSec = computed(() => store.settings.monitorIntervalSec)

const info = ref<Sample | null>(null)
const samples = ref<Sample[]>([])

const cards = computed(() => {
  const s = info.value
  if (!s) return []
  const hitColor = s.hitRate >= 80 ? 'var(--green)' : s.hitRate >= 50 ? 'var(--amber)' : 'var(--red)'
  return [
    { label: 'QPS', value: fmtNum(Math.round(s.qps)), sub: '每秒命令数', color: 'var(--red)' },
    { label: '命中率', value: s.hitRate.toFixed(1) + '%', sub: `命中 ${fmtNum(Math.round(s.hitsRate))}/s`, color: hitColor },
    { label: '内存', value: fmtBytes(s.usedMem), sub: s.maxMem > 0 ? `上限 ${fmtBytes(s.maxMem)}` : '未设上限', color: 'var(--t-hash)' },
    { label: '碎片率', value: s.fragRatio > 0 ? s.fragRatio.toFixed(2) : '-', sub: s.fragRatio > 1.5 ? '偏高' : '正常', color: s.fragRatio > 1.5 ? 'var(--amber)' : 'var(--green)' },
    { label: '连接数', value: String(s.clients), sub: `阻塞 ${s.blocked}`, color: 'var(--t-stream)' },
    { label: '键总数', value: fmtNum(s.totalKeys), sub: `过期 ${fmtNum(Math.round(s.expiredRate))}/s`, color: 'var(--t-list)' },
    { label: '网络入', value: fmtBytes(Math.round(s.netInRate)) + '/s', sub: '', color: 'var(--t-zset)' },
    { label: '网络出', value: fmtBytes(Math.round(s.netOutRate)) + '/s', sub: s.slaves > 0 ? `从节点 ${s.slaves}` : '', color: 'var(--t-set)' },
  ]
})

// ---------- ECharts ----------
const qpsChart = ref<HTMLElement>()
const memChart = ref<HTMLElement>()
const netChart = ref<HTMLElement>()
const connChart = ref<HTMLElement>()
let charts: echarts.ECharts[] = []
const chartThemes = reactive({
  text: '#5b6472',
  border: '#e4e7ee',
  split: '#eef0f4',
})

function baseOpts() {
  const dark = store.settings.theme === 'dark'
  return {
    animation: false,
    grid: { left: 56, right: 16, top: 30, bottom: 40 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: dark ? '#262b34' : '#fff',
      borderColor: dark ? '#333a46' : '#e4e7ee',
      textStyle: { color: dark ? '#e6e9ef' : '#1f2430', fontSize: 12, fontFamily: 'JetBrains Mono' },
    },
    legend: { textStyle: { color: dark ? '#a8b0bd' : '#5b6472', fontSize: 11 }, top: 0, itemWidth: 14 },
    xAxis: {
      type: 'time',
      axisLabel: { color: chartThemes.text, fontSize: 10.5, hideOverlap: true },
      splitLine: { show: false },
      axisLine: { lineStyle: { color: chartThemes.border } },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: chartThemes.text, fontSize: 10.5 },
      splitLine: { lineStyle: { color: chartThemes.split } },
    },
    dataZoom: [{ type: 'inside' }, { type: 'slider', height: 18, bottom: 6 }],
  }
}

function renderAll() {
  const data = samples.value
  const t = (s: Sample) => s.time
  updateChart(qpsChart.value, [
    { name: 'QPS', type: 'line', showSymbol: false, data: data.map((s) => [t(s), Math.round(s.qps)]), itemStyle: { color: '#dc382d' }, areaStyle: { opacity: 0.08 } },
    { name: '命中率%', type: 'line', showSymbol: false, data: data.map((s) => [t(s), +s.hitRate.toFixed(1)]), itemStyle: { color: '#16a34a' }, yAxisIndex: 0 },
  ], { yAxis: [{ type: 'value', axisLabel: { color: chartThemes.text, fontSize: 10.5 }, splitLine: { lineStyle: { color: chartThemes.split } } }] })

  updateChart(memChart.value, [
    { name: '已用内存', type: 'line', showSymbol: false, data: data.map((s) => [t(s), Math.round(s.usedMem / 1024)]), itemStyle: { color: '#3b82f6' }, areaStyle: { opacity: 0.08 } },
    { name: 'RSS', type: 'line', showSymbol: false, data: data.map((s) => [t(s), Math.round(s.rssMem / 1024)]), itemStyle: { color: '#a855f7' } },
    { name: '碎片率', type: 'line', showSymbol: false, data: data.map((s) => [t(s), +s.fragRatio.toFixed(2)]), itemStyle: { color: '#f59e0b' } },
  ], { yAxis: { type: 'value', name: 'KB', axisLabel: { color: chartThemes.text, fontSize: 10.5 }, splitLine: { lineStyle: { color: chartThemes.split } } } })

  updateChart(netChart.value, [
    { name: '入 (KB/s)', type: 'line', showSymbol: false, data: data.map((s) => [t(s), +(s.netInRate / 1024).toFixed(1)]), itemStyle: { color: '#06b6d4' }, areaStyle: { opacity: 0.08 } },
    { name: '出 (KB/s)', type: 'line', showSymbol: false, data: data.map((s) => [t(s), +(s.netOutRate / 1024).toFixed(1)]), itemStyle: { color: '#22c55e' } },
  ])

  updateChart(connChart.value, [
    { name: '连接数', type: 'line', showSymbol: false, step: 'end', data: data.map((s) => [t(s), s.clients]), itemStyle: { color: '#3b82f6' } },
    { name: '阻塞', type: 'line', showSymbol: false, step: 'end', data: data.map((s) => [t(s), s.blocked]), itemStyle: { color: '#f59e0b' } },
  ])
}

function updateChart(el: HTMLElement | undefined, series: any[], overrides: any = {}) {
  if (!el) return
  let chart = charts.find((c) => (c.getDom() as HTMLElement) === el)
  if (!chart) {
    chart = echarts.init(el)
    charts.push(chart)
  }
  const opts: any = baseOpts()
  Object.assign(opts, overrides)
  opts.series = series
  chart.setOption(opts, true)
}

async function loadHistory() {
  const c = conn.value
  if (!c) return
  try {
    samples.value = await api.monitorHistory(c.id, windowSec.value)
    if (samples.value.length) info.value = samples.value[samples.value.length - 1]
    renderAll()
  } catch {
    // 连接刚断开时静默
  }
}

function setWindow(sec: number) {
  windowSec.value = sec
  loadHistory()
}

// 实时数据驱动
let lastCount = 0
let raf = 0
function tick() {
  const c = conn.value
  if (c && live.value) {
    const arr = samplesOf(c.id)
    if (arr.length > 0) {
      info.value = arr[arr.length - 1]
      const cutoff = Date.now() - windowSec.value * 1000
      const visible = arr.filter((s) => s.time >= cutoff)
      if (visible.length !== lastCount) {
        lastCount = visible.length
        samples.value = visible
        renderAll()
      }
    }
  }
  raf = window.setTimeout(tick, 2000) as unknown as number
}

watch(() => store.settings.theme, () => {
  const dark = store.settings.theme === 'dark'
  chartThemes.text = dark ? '#a8b0bd' : '#5b6472'
  chartThemes.border = dark ? '#333a46' : '#e4e7ee'
  chartThemes.split = dark ? '#2e3440' : '#eef0f4'
  charts.forEach((c) => c.dispose())
  charts = []
  renderAll()
})

watch(() => conn.value?.id, () => {
  samples.value = []
  lastCount = 0
  loadHistory()
})

onMounted(() => {
  loadHistory()
  tick()
  window.addEventListener('resize', resizeAll)
})
onBeforeUnmount(() => {
  clearTimeout(raf)
  window.removeEventListener('resize', resizeAll)
  charts.forEach((c) => c.dispose())
  charts = []
})

function resizeAll() {
  charts.forEach((c) => c.resize())
}
</script>

<style scoped>
.dash {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.dash-head { display: flex; align-items: center; gap: 12px; }
.dash-head h2 { margin: 0; font-size: 17px; font-weight: 700; }
.window-picker { display: flex; gap: 4px; }
.live-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--text-2);
}
.live-toggle input { accent-color: var(--red); }

.metric-cards {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 10px;
}
.metric {
  padding: 12px 14px;
  min-width: 0;
}
.m-label { font-size: 11px; color: var(--text-3); font-weight: 600; }
.m-value { font-size: 19px; font-weight: 700; margin-top: 4px; white-space: nowrap; }
.m-sub { font-size: 11px; margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.charts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  flex: 1;
  min-height: 0;
}
.chart-card {
  padding: 14px;
  display: flex;
  flex-direction: column;
  min-height: 260px;
}
.chart-card h4 { margin: 0 0 8px; font-size: 13px; color: var(--text-2); font-weight: 600; }
.chart { flex: 1; min-height: 200px; }
</style>
