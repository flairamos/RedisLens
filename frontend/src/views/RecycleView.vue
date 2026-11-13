<template>
  <div class="recycle">
    <div class="rc-head">
      <h2>回收站</h2>
      <span class="faint" style="font-size: 12.5px">删除的键在此保留（容量上限 100 条 / 64MB，超出自动淘汰最旧）</span>
      <span class="grow"></span>
      <button class="btn danger-ghost" @click="clearAll" :disabled="!items.length || conn?.readOnly">
        <Icon name="trash" :size="14" /> 清空
      </button>
    </div>

    <div class="card" v-if="items.length">
      <table class="tbl">
        <thead>
          <tr>
            <th>键名</th>
            <th style="width: 80px">类型</th>
            <th style="width: 90px">TTL</th>
            <th style="width: 90px">大小</th>
            <th style="width: 160px">删除时间</th>
            <th style="width: 150px"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="it in items" :key="it.token">
            <td class="mono ellipsis" :title="it.key">{{ it.key }}</td>
            <td><TypeBadge :type="it.type" /></td>
            <td class="mono faint">{{ fmtTTL(it.pttl) }}</td>
            <td class="mono faint">{{ fmtBytes(it.size) }}</td>
            <td class="mono faint">{{ new Date(it.deletedAt).toLocaleString('zh-CN', { hour12: false }) }}</td>
            <td>
              <button class="btn sm primary" :disabled="conn?.readOnly" @click="restore(it)">
                <Icon name="refresh" :size="13" /> 恢复
              </button>
              <button class="btn sm danger-ghost" style="margin-left: 6px" @click="remove(it)" title="彻底删除备份">
                <Icon name="trash" :size="13" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="empty" style="flex: 1">
      <Icon name="trash" :size="44" />
      <div>回收站为空</div>
      <div class="faint" style="font-size: 12.5px">删除键时会自动 DUMP 备份到此处，支持一键恢复</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import TypeBadge from '../components/TypeBadge.vue'
import { currentConn, store, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import { fmtBytes, fmtTTL } from '../lib/format'
import type { RecycleItem } from '../lib/types'

const conn = computed(() => currentConn())
const items = ref<RecycleItem[]>([])

async function load() {
  const c = conn.value
  if (!c) return
  try {
    items.value = await api.recycleList(c.id)
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function restore(it: RecycleItem) {
  const c = conn.value
  if (!c) return
  try {
    await api.recycleRestore(c.id, it.token)
    toast('ok', `键 ${it.key} 已恢复`)
    await load()
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function remove(it: RecycleItem) {
  const c = conn.value
  if (!c) return
  try {
    await api.recycleRemove(c.id, it.token)
    await load()
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function clearAll() {
  const c = conn.value
  if (!c) return
  try {
    await api.recycleClear(c.id)
    toast('ok', '回收站已清空')
    await load()
  } catch (e) {
    toast('err', errMsg(e))
  }
}

onMounted(load)
</script>

<style scoped>
.recycle {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.rc-head { display: flex; align-items: center; gap: 12px; }
.rc-head h2 { margin: 0; font-size: 17px; font-weight: 700; }
</style>
