<template>
  <div class="settings">
    <div class="st-head">
      <h2>设置</h2>
    </div>

    <div class="card st-card">
      <h4>外观</h4>
      <div class="st-row">
        <span class="st-label">主题</span>
        <div class="seg">
          <button :class="{ on: settings.theme === 'light' }" @click="set({ theme: 'light' })">浅色</button>
          <button :class="{ on: settings.theme === 'dark' }" @click="set({ theme: 'dark' })">深色</button>
        </div>
      </div>
      <div class="st-row">
        <span class="st-label">列表密度</span>
        <div class="seg">
          <button :class="{ on: settings.density === 'compact' }" @click="set({ density: 'compact' })">紧凑</button>
          <button :class="{ on: settings.density === 'comfortable' }" @click="set({ density: 'comfortable' })">舒适</button>
          <button :class="{ on: settings.density === 'spacious' }" @click="set({ density: 'spacious' })">宽松</button>
        </div>
      </div>
    </div>

    <div class="card st-card">
      <h4>监控</h4>
      <div class="st-row">
        <span class="st-label">采样间隔（秒）</span>
        <input v-model.number="settings.monitorIntervalSec" type="number" min="1" max="60" style="width: 90px" @change="save" />
      </div>
    </div>

    <div class="card st-card">
      <h4>安全</h4>
      <div class="st-row">
        <span class="st-label">危险命令二次确认</span>
        <label class="chk-row">
          <input type="checkbox" v-model="settings.confirmDanger" @change="save" />
        </label>
      </div>
      <p class="st-hint faint">
        拦截范围：FLUSHALL / FLUSHDB / SHUTDOWN / CONFIG / DEBUG / SLAVEOF / REPLICAOF / SWAPDB /
        MIGRATE / KEYS / MONITOR / SCRIPT FLUSH / FUNCTION FLUSH / CLUSTER 写操作 / ACL 写操作等。
        控制台与可视化操作两个入口统一拦截并记入审计日志。
      </p>
    </div>

    <div class="card st-card">
      <h4>数据</h4>
      <div class="st-row">
        <span class="st-label">SCAN COUNT</span>
        <input v-model.number="settings.scanCount" type="number" min="10" style="width: 100px" @change="save" />
      </div>
      <div class="st-row">
        <span class="st-label">本地日志开关</span>
        <label class="chk-row">
          <input type="checkbox" v-model="settings.logEnabled" @change="save" />
        </label>
      </div>
    </div>

    <div class="card st-card" v-if="appInfo">
      <h4>关于</h4>
      <div class="st-row"><span class="st-label">版本</span><span class="mono">v{{ appInfo.version }}</span></div>
      <div class="st-row"><span class="st-label">数据目录</span><span class="mono faint" style="font-size: 12px">{{ appInfo.dataDir }}</span></div>
      <div class="st-row"><span class="st-label">启动时间</span><span class="mono">{{ appInfo.startedAt }}</span></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { store, saveSettings, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import type { Settings } from '../lib/types'

const settings = reactive<Settings>({ ...store.settings })
const appInfo = ref<Record<string, any> | null>(null)

function set(patch: Partial<Settings>) {
  Object.assign(settings, patch)
  save()
}

async function save() {
  try {
    await saveSettings(settings as Settings)
    toast('ok', '设置已保存')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

onMounted(async () => {
  try {
    appInfo.value = await api.getAppInfo()
  } catch {
    /* ignore */
  }
})
</script>

<style scoped>
.settings {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 720px;
}
.st-head h2 { margin: 0; font-size: 17px; font-weight: 700; }
.st-card { padding: 16px 20px; }
.st-card h4 { margin: 0 0 12px; font-size: 13px; color: var(--text-2); text-transform: uppercase; letter-spacing: 0.5px; }
.st-row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 0;
}
.st-label { width: 170px; flex-shrink: 0; font-size: 13px; font-weight: 500; }
.seg {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}
.seg button {
  padding: 6px 16px;
  font-size: 12.5px;
  color: var(--text-2);
  border-right: 1px solid var(--border);
}
.seg button:last-child { border-right: none; }
.seg button.on { background: var(--red); color: #fff; font-weight: 600; }
.st-hint { margin: 6px 0 0; font-size: 12px; line-height: 1.6; }
.chk-row input { width: 16px; height: 16px; accent-color: var(--red); }
</style>
