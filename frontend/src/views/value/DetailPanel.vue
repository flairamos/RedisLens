<template>
  <aside class="detail" v-if="vd">
    <header class="dt-head">
      <Icon name="layers" :size="14" />
      <span>键详情</span>
      <button class="icon-btn" @click="store.detailVisible = false" title="收起">
        <Icon name="chevron-right" :size="14" />
      </button>
    </header>

    <div class="dt-body">
      <dl class="dt-meta">
        <div class="dt-row"><dt>键名</dt><dd class="mono ellipsis" :title="vd.key">{{ vd.key }}</dd></div>
        <div class="dt-row"><dt>类型</dt><dd><TypeBadge :type="vd.type" /></dd></div>
        <div class="dt-row"><dt>编码</dt><dd class="mono">{{ vd.encoding || '-' }}</dd></div>
        <div class="dt-row"><dt>大小</dt><dd class="mono">{{ fmtBytes(vd.size) }}</dd></div>
        <div class="dt-row">
          <dt>元素</dt>
          <dd class="mono">{{ vd.type === 'string' || vd.type === 'json' ? fmtBytes(vd.elements) : vd.elements.toLocaleString() }}</dd>
        </div>
        <div class="dt-row"><dt>TTL</dt><dd class="mono">{{ fmtTTL(vd.ttl) }}</dd></div>
      </dl>

      <!-- TTL 编辑 -->
      <div class="dt-section" v-if="!readOnly">
        <h4>TTL 管理</h4>
        <div class="ttl-row">
          <input v-model="ttlSec" type="number" placeholder="秒" min="-1" />
          <button class="btn sm primary" @click="applyTTL" :disabled="!ttlSec">续期</button>
          <button class="btn sm" @click="persist" title="移除过期时间">永久</button>
        </div>
      </div>

      <!-- 操作 -->
      <div class="dt-section" v-if="!readOnly">
        <h4>操作</h4>
        <div class="dt-actions">
          <button class="btn sm" @click="startRename"><Icon name="edit" :size="13" /> 重命名</button>
          <button class="btn sm danger-ghost" @click="del"><Icon name="trash" :size="13" /> 删除键</button>
        </div>
      </div>

      <!-- 只读提示 -->
      <div v-if="readOnly" class="dt-ro">
        <Icon name="lock" :size="13" />
        只读模式下写操作已隐藏
      </div>
    </div>

    <!-- 重命名对话框 -->
    <Modal :visible="renameOpen" title="重命名键" @close="renameOpen = false">
      <div class="form-col">
        <label>新键名</label>
        <input v-model="renameBuf" class="mono" spellcheck="false" @keyup.enter="doRename" />
      </div>
      <template #footer>
        <button class="btn" @click="renameOpen = false">取消</button>
        <button class="btn primary" @click="doRename">确定</button>
      </template>
    </Modal>

    <!-- 删除确认 -->
    <ConfirmDialog
      :visible="delConfirm"
      title="删除键"
      :danger="true"
      @confirm="doDel"
      @cancel="delConfirm = false"
    >
      将删除键 <code class="mono">{{ vd.key }}</code>（类型 {{ vd.type }}）。删除前会自动备份到回收站，可恢复。
    </ConfirmDialog>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../../components/Icon.vue'
import TypeBadge from '../../components/TypeBadge.vue'
import Modal from '../../components/Modal.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'
import { currentConn, store, toast } from '../../lib/store'
import { api, errMsg } from '../../lib/api'
import { fmtBytes, fmtTTL } from '../../lib/format'
import type { ValueData } from '../../lib/types'

const props = defineProps<{ valueData: ValueData; readOnly: boolean }>()
const emit = defineEmits<{ (e: 'refresh'): void; (e: 'deleted'): void }>()

const vd = computed(() => props.valueData)
const del = () => (delConfirm.value = true)

const ttlSec = ref('')
const renameOpen = ref(false)
const renameBuf = ref('')
const delConfirm = ref(false)

async function applyTTL() {
  const conn = currentConn()
  if (!conn) return
  const sec = parseInt(ttlSec.value)
  if (isNaN(sec) || sec <= 0) {
    toast('err', '请输入大于 0 的秒数')
    return
  }
  try {
    await api.setTTL(conn.id, store.currentDB, [props.valueData.key], sec * 1000)
    toast('ok', `TTL 已设置为 ${sec}s`)
    ttlSec.value = ''
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function persist() {
  const conn = currentConn()
  if (!conn) return
  try {
    await api.setTTL(conn.id, store.currentDB, [props.valueData.key], -2)
    toast('ok', '已移除 TTL（永久）')
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

function startRename() {
  renameBuf.value = props.valueData.key
  renameOpen.value = true
}

async function doRename() {
  const conn = currentConn()
  if (!conn || !renameBuf.value || renameBuf.value === props.valueData.key) return
  try {
    await api.renameKey(conn.id, store.currentDB, props.valueData.key, renameBuf.value)
    toast('ok', '已重命名')
    renameOpen.value = false
    emit('refresh')
  } catch (e) {
    toast('err', errMsg(e))
  }
}

async function doDel() {
  const conn = currentConn()
  delConfirm.value = false
  if (!conn) return
  try {
    const r = await api.deleteKeys(conn.id, store.currentDB, [props.valueData.key], true)
    toast('ok', `键已删除（回收站可恢复）`)
    emit('deleted')
  } catch (e) {
    toast('err', errMsg(e))
  }
}
</script>

<style scoped>
.detail {
  width: var(--detail-w);
  background: var(--bg-panel);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow-y: auto;
}
.dt-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border);
  font-weight: 600;
  font-size: 13px;
}
.dt-head .icon-btn { margin-left: auto; }
.dt-body { padding: 14px; display: flex; flex-direction: column; gap: 16px; }
.dt-meta { margin: 0; display: flex; flex-direction: column; gap: 8px; }
.dt-row { display: flex; gap: 10px; align-items: baseline; }
.dt-row dt {
  width: 52px;
  flex-shrink: 0;
  color: var(--text-3);
  font-size: 12px;
}
.dt-row dd { margin: 0; font-size: 12.5px; min-width: 0; }
.dt-section h4 {
  margin: 0 0 8px;
  font-size: 11.5px;
  color: var(--text-3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.ttl-row { display: flex; gap: 6px; }
.ttl-row input { width: 90px; }
.dt-actions { display: flex; gap: 8px; }
.dt-ro {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-3);
  font-size: 12px;
  padding: 10px;
  background: var(--bg-hover);
  border-radius: var(--radius-sm);
}
.form-col { display: flex; flex-direction: column; gap: 6px; }
.form-col label { font-size: 12px; color: var(--text-2); font-weight: 600; }
</style>
