<template>
  <div class="dataview">
    <div class="dv-head">
      <h2>导入导出与批量删除</h2>
    </div>

    <div class="dv-grid">
      <!-- 导出 -->
      <div class="card dv-card">
        <h4><Icon name="download" :size="15" /> 导出键</h4>
        <div class="form-col">
          <label>匹配模式</label>
          <input v-model="match" class="mono" placeholder="*" spellcheck="false" />
          <label>类型筛选</label>
          <select v-model="typeFilter">
            <option value="">全部</option>
            <option v-for="t in types" :key="t" :value="t">{{ t }}</option>
          </select>
          <div class="btn-row">
            <button class="btn primary" :disabled="busy" @click="doExport('json')">
              <Icon name="save" :size="14" /> 导出 JSON（含值）
            </button>
            <button class="btn" :disabled="busy" @click="doExport('csv')">
              导出 CSV 清单
            </button>
          </div>
        </div>
      </div>

      <!-- 导入 -->
      <div class="card dv-card">
        <h4><Icon name="upload" :size="15" /> 导入键</h4>
        <div class="form-col">
          <label class="faint" style="font-weight: 400">
            支持本工具导出的 JSON 格式（含 DUMP 保真数据，RESTORE 恢复，可跨实例迁移）。
          </label>
          <label class="chk-row">
            <input type="checkbox" v-model="overwrite" />
            覆盖已存在的键
          </label>
          <button class="btn primary" :disabled="busy || conn?.readOnly" @click="doImport">
            <Icon name="upload" :size="14" /> 选择文件并导入
          </button>
        </div>
      </div>

      <!-- 批量删除 -->
      <div class="card dv-card danger-card">
        <h4 style="color: var(--red)"><Icon name="trash" :size="15" /> 批量删除（预览确认）</h4>
        <div class="form-col">
          <label>匹配模式</label>
          <div class="btn-row">
            <input v-model="delMatch" class="mono grow" placeholder="cache:temp:*" spellcheck="false" />
            <select v-model="delType" style="width: 110px">
              <option value="">全部</option>
              <option v-for="t in types" :key="t" :value="t">{{ t }}</option>
            </select>
          </div>
          <button class="btn" :disabled="busy || conn?.readOnly" @click="doPreview">
            <Icon name="search" :size="14" /> 预览匹配键
          </button>
        </div>

        <div v-if="preview" class="preview-area">
          <div class="preview-warn">
            <Icon name="alert" :size="15" />
            将删除 <b>{{ preview.total.toLocaleString() }}</b> 个键（匹配 <code class="mono">{{ preview.match }}</code>）。
            删除前自动备份至回收站。
          </div>
          <table class="tbl" style="max-height: 200px; overflow-y: auto; display: block">
            <thead><tr><th>键名（前 1000 个）</th><th style="width: 76px">类型</th><th style="width: 90px">TTL</th></tr></thead>
            <tbody>
              <tr v-for="k in preview.samples.slice(0, 1000)" :key="k.key">
                <td class="mono ellipsis">{{ k.key }}</td>
                <td><TypeBadge :type="k.type" /></td>
                <td class="mono faint">{{ fmtTTL(k.ttl) }}</td>
              </tr>
            </tbody>
          </table>
          <button class="btn danger" :disabled="preview.total === 0" @click="doBatchDelete">
            <Icon name="trash" :size="14" /> 确认删除 {{ preview.total.toLocaleString() }} 键
          </button>
        </div>
      </div>
    </div>

    <!-- 结果 -->
    <div v-if="lastReport" class="card report-card">
      <h4><Icon name="check" :size="15" /> 操作完成</h4>
      <pre class="mono report-text">{{ lastReport }}</pre>
    </div>

    <ConfirmDialog
      :visible="confirmDel"
      :title="`批量删除 ${preview?.total.toLocaleString()} 个键`"
      :danger="true"
      :require-text="preview?.match || ''"
      confirm-text="永久删除（已备份回收站）"
      @confirm="confirmBatchDelete"
      @cancel="confirmDel = false"
    >
      即将从 <b>{{ conn?.name }}</b> 删除匹配 <code class="mono">{{ preview?.match }}</code> 的
      {{ preview?.total.toLocaleString() }} 个键。请输入匹配模式以确认。
    </ConfirmDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../components/Icon.vue'
import TypeBadge from '../components/TypeBadge.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { store, currentConn, toast } from '../lib/store'
import { api, errMsg } from '../lib/api'
import { fmtTTL } from '../lib/format'
import type { DeletePreview } from '../lib/types'

const conn = computed(() => currentConn())
const types = ['string', 'hash', 'list', 'set', 'zset', 'stream', 'json']

const match = ref('*')
const typeFilter = ref('')
const delMatch = ref('')
const delType = ref('')
const overwrite = ref(true)
const busy = ref(false)
const preview = ref<DeletePreview | null>(null)
const confirmDel = ref(false)
const lastReport = ref('')

async function doExport(kind: 'json' | 'csv') {
  const c = conn.value
  if (!c) return
  const file = await api.pickSaveFile('导出键', kind === 'json' ? 'redislens-export.json' : 'redislens-keys.csv')
  if (!file) return
  busy.value = true
  try {
    const r = kind === 'json'
      ? await api.exportKeys(c.id, store.currentDB, match.value, typeFilter.value, file)
      : await api.exportCSV(c.id, store.currentDB, match.value, typeFilter.value, file)
    lastReport.value = `导出完成\n文件: ${r.file}\n键数: ${r.keys}\n耗时: ${r.durationMs}ms`
    toast('ok', `已导出 ${r.keys} 键`)
  } catch (e) {
    toast('err', errMsg(e))
  } finally {
    busy.value = false
  }
}

async function doImport() {
  const c = conn.value
  if (!c) return
  const file = await api.pickOpenFile('选择导出文件')
  if (!file) return
  busy.value = true
  try {
    const r = await api.importFile(c.id, store.currentDB, file, overwrite.value)
    lastReport.value = `导入完成\n文件: ${r.file}\n总计: ${r.total}\n成功: ${r.imported}\n跳过: ${r.skipped}\n失败: ${r.failed}\n耗时: ${r.durationMs}ms`
    toast('ok', `导入 ${r.imported} 键（跳过 ${r.skipped}，失败 ${r.failed}）`)
  } catch (e) {
    toast('err', errMsg(e))
  } finally {
    busy.value = false
  }
}

async function doPreview() {
  const c = conn.value
  if (!c || !delMatch.value) return
  busy.value = true
  try {
    preview.value = await api.previewDelete(c.id, store.currentDB, delMatch.value, delType.value, 1000)
  } catch (e) {
    toast('err', errMsg(e))
  } finally {
    busy.value = false
  }
}

function doBatchDelete() {
  confirmDel.value = true
}

async function confirmBatchDelete() {
  const c = conn.value
  confirmDel.value = false
  if (!c || !preview.value) return
  busy.value = true
  try {
    // 分批删除（每批 500），全部备份
    const keys = preview.value.samples.map((k) => k.key)
    let deleted = 0
    for (let i = 0; i < keys.length; i += 500) {
      const r = await api.deleteKeys(c.id, store.currentDB, keys.slice(i, i + 500), true)
      deleted += r.deleted
    }
    toast('ok', `已删除 ${deleted} 个键（回收站可恢复）`)
    lastReport.value = `批量删除完成\n匹配: ${preview.value.match}\n删除: ${deleted} 键（全部备份至回收站）`
    preview.value = null
  } catch (e) {
    toast('err', errMsg(e))
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.dataview {
  padding: 16px 20px;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.dv-head h2 { margin: 0; font-size: 17px; font-weight: 700; }

.dv-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.dv-card { padding: 16px; }
.dv-card h4 {
  margin: 0 0 12px;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.danger-card { grid-column: 1 / -1; border-top: 2px solid var(--red); }

.form-col { display: flex; flex-direction: column; gap: 10px; }
.form-col label { font-size: 12.5px; color: var(--text-2); font-weight: 600; }
.btn-row { display: flex; gap: 8px; align-items: center; }
.chk-row { display: flex; align-items: center; gap: 6px; cursor: pointer; font-weight: 500 !important; }
.chk-row input { accent-color: var(--red); }

.preview-area { margin-top: 14px; display: flex; flex-direction: column; gap: 10px; }
.preview-warn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: var(--danger-bg);
  color: var(--red);
  border-radius: var(--radius-sm);
  font-size: 13px;
}
.preview-warn code {
  font-family: var(--font-mono);
  background: rgba(220, 56, 45, 0.15);
  padding: 1px 6px;
  border-radius: 4px;
}
.preview-area .btn.danger { align-self: flex-start; }

.report-card { padding: 14px 16px; }
.report-card h4 { margin: 0 0 8px; font-size: 13.5px; color: var(--green); display: flex; align-items: center; gap: 6px; }
.report-text { margin: 0; font-size: 12.5px; color: var(--text-2); white-space: pre-wrap; }
</style>
