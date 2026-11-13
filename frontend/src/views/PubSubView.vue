<template>
  <div class="pubsub">
    <div class="ps-head">
      <h2>发布订阅</h2>
      <span class="grow"></span>
      <button v-if="!subscribed" class="btn primary" :disabled="!channelsText && !patternsText" @click="subscribe">
        <Icon name="radio" :size="15" /> 订阅
      </button>
      <button v-else class="btn danger-ghost" @click="unsubscribe">
        <Icon name="stop" :size="14" /> 退订
      </button>
    </div>

    <div class="ps-config card">
      <div class="ps-row">
        <label>频道（逗号分隔）</label>
        <input v-model="channelsText" class="mono" placeholder="news, chat:room:1" spellcheck="false" :disabled="subscribed" />
      </div>
      <div class="ps-row">
        <label>模式订阅（逗号分隔）</label>
        <input v-model="patternsText" class="mono" placeholder="news.*, chat:*" spellcheck="false" :disabled="subscribed" />
      </div>
    </div>

    <div class="ps-main">
      <!-- 消息流 -->
      <div class="ps-stream card">
        <div class="ps-stream-head">
          <span class="faint">实时消息</span>
          <span class="grow"></span>
          <label class="faint" style="display: flex; align-items: center; gap: 4px">
            <input type="checkbox" v-model="pause" /> 暂停接收
          </label>
          <span class="tag" v-if="subscribed" style="background: rgba(22,163,74,0.12); color: var(--green)">订阅中 · {{ msgCount }} 条</span>
          <button class="btn sm ghost" @click="messages = []"><Icon name="trash" :size="13" /></button>
        </div>
        <div class="ps-msgs" ref="msgsEl">
          <div v-for="(m, i) in messages" :key="i" class="ps-msg">
            <span class="ps-time faint mono">{{ fmtTimeMs(m.ts) }}</span>
            <span
              class="ps-kind"
              :style="{ color: m.kind.includes('message') ? 'var(--t-hash)' : 'var(--t-none)' }"
            >{{ m.kind }}</span>
            <span class="mono ps-channel">{{ m.channel }}</span>
            <span v-if="m.pattern" class="faint mono">({{ m.pattern }})</span>
            <span class="mono ps-payload" :class="{ binary: m.binary }">{{ m.payload }}</span>
          </div>
          <div v-if="!messages.length" class="empty" style="flex: 1">
            <Icon name="radio" :size="36" />
            <div>订阅频道后，消息将实时滚动显示</div>
          </div>
        </div>
      </div>

      <!-- 发布 -->
      <div class="ps-publish card">
        <h4>发布消息</h4>
        <div class="form-col">
          <label>频道</label>
          <input v-model="pubChannel" class="mono" spellcheck="false" :disabled="conn?.readOnly" />
          <label>负载</label>
          <textarea v-model="pubPayload" rows="5" class="mono" spellcheck="false" :disabled="conn?.readOnly"></textarea>
          <button class="btn primary" :disabled="!pubChannel || conn?.readOnly" @click="publish">
            <Icon name="send" :size="14" /> 发布
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, nextTick } from 'vue'
import Icon from '../components/Icon.vue'
import { store, currentConn, toast } from '../lib/store'
import { api, errMsg, onEvent } from '../lib/api'
import { fmtTimeMs } from '../lib/format'
import type { PubSubMessage } from '../lib/types'

const conn = computed(() => currentConn())
const channelsText = ref('')
const patternsText = ref('')
const subscribed = ref(false)
const messages = ref<PubSubMessage[]>([])
const msgCount = computed(() => messages.value.length)
const pause = ref(false)
const msgsEl = ref<HTMLElement>()
const pubChannel = ref('')
const pubPayload = ref('')

const buffer: PubSubMessage[] = []
let flushTimer: number | undefined

function flush() {
  if (pause.value || buffer.length === 0) return
  const batch = buffer.splice(0)
  messages.value.push(...batch)
  if (messages.value.length > 1000) messages.value.splice(0, messages.value.length - 1000)
  nextTick(() => {
    if (msgsEl.value) msgsEl.value.scrollTop = msgsEl.value.scrollHeight
  })
}

async function subscribe() {
  const c = conn.value
  if (!c) return
  const channels = channelsText.value.split(/[,，]/).map((s) => s.trim()).filter(Boolean)
  const patterns = patternsText.value.split(/[,，]/).map((s) => s.trim()).filter(Boolean)
  if (!channels.length && !patterns.length) return
  try {
    await api.pubsubSubscribe(c.id, channels, patterns)
    subscribed.value = true
    toast('ok', `已订阅 ${channels.length} 频道 / ${patterns.length} 模式`)
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function unsubscribe() {
  const c = conn.value
  if (!c) return
  try {
    await api.pubsubUnsubscribe(c.id)
    subscribed.value = false
    toast('ok', '已退订')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function publish() {
  const c = conn.value
  if (!c || !pubChannel.value) return
  try {
    const n = await api.publish(c.id, pubChannel.value, pubPayload.value)
    toast('ok', `已发布至 ${n} 个订阅者`)
  } catch (e) {
    toast('err', errMsg(e))
  }
}

let offMsg: (() => void) | null = null
onMounted(() => {
  offMsg = onEvent('pubsub:message', (m: PubSubMessage) => {
    if (m.connId !== store.currentConnId) return
    if (m.kind === 'subscribe' || m.kind === 'psubscribe') {
      messages.value.push(m)
      return
    }
    buffer.push(m)
  }) as any
  flushTimer = window.setInterval(flush, 300)
})
onBeforeUnmount(() => {
  offMsg?.()
  clearInterval(flushTimer)
  if (subscribed.value) api.pubsubUnsubscribe(store.currentConnId).catch(() => {})
})

watch(() => conn.value?.id, () => {
  messages.value = []
  subscribed.value = false
})
</script>

<style scoped>
.pubsub {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: 100%;
  overflow: hidden;
}
.ps-head { display: flex; align-items: center; gap: 12px; }
.ps-head h2 { margin: 0; font-size: 17px; font-weight: 700; }

.ps-config { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
.ps-row { display: flex; align-items: center; gap: 12px; }
.ps-row label { width: 130px; flex-shrink: 0; font-size: 12.5px; color: var(--text-2); font-weight: 600; }
.ps-row input { flex: 1; }

.ps-main {
  flex: 1;
  display: flex;
  gap: 12px;
  min-height: 0;
}
.ps-stream {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.ps-stream-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  font-size: 12.5px;
}
.ps-msgs { flex: 1; overflow-y: auto; padding: 10px 14px; display: flex; flex-direction: column; gap: 4px; }
.ps-msg { display: flex; gap: 10px; align-items: baseline; font-size: 12.5px; min-width: 0; }
.ps-time { font-size: 10.5px; width: 66px; flex-shrink: 0; }
.ps-kind { font-size: 10.5px; font-weight: 700; width: 70px; flex-shrink: 0; text-transform: uppercase; }
.ps-channel { color: var(--t-stream); font-weight: 600; flex-shrink: 0; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ps-payload { flex: 1; min-width: 0; white-space: pre-wrap; word-break: break-all; color: var(--text-2); }
.ps-payload.binary { color: var(--text-3); font-style: italic; }

.ps-publish {
  width: 300px;
  flex-shrink: 0;
  padding: 14px 16px;
}
.ps-publish h4 { margin: 0 0 12px; font-size: 13.5px; }
.form-col { display: flex; flex-direction: column; gap: 8px; }
.form-col label { font-size: 12px; color: var(--text-2); font-weight: 600; }
</style>
