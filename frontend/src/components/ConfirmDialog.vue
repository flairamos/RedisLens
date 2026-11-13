<template>
  <Teleport to="body">
    <div v-if="visible" class="confirm-mask" @click.self="emit('cancel')">
      <div class="confirm-card" :class="{ danger }">
        <div class="confirm-icon">
          <Icon :name="danger ? 'alert' : 'info'" :size="26" />
        </div>
        <h3>{{ title }}</h3>
        <p class="confirm-msg"><slot /></p>
        <div v-if="requireText" class="confirm-input">
          <label>输入 <code>{{ requireText }}</code> 以确认：</label>
          <input v-model="typed" :placeholder="requireText" spellcheck="false" />
        </div>
        <div class="confirm-actions">
          <button class="btn" @click="emit('cancel')">取消</button>
          <button
            class="btn danger"
            :disabled="requireText ? typed !== requireText : false"
            @click="emit('confirm')"
          >
            {{ confirmText || '确认执行' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import Icon from './Icon.vue'

const props = defineProps<{
  visible: boolean
  title: string
  danger?: boolean
  confirmText?: string
  requireText?: string // 输入确认文本（FLUSHALL 等高危操作）
}>()
const emit = defineEmits<{ (e: 'confirm'): void; (e: 'cancel'): void }>()

const typed = ref('')
watch(() => props.visible, (v) => {
  if (v) typed.value = ''
})
</script>

<style scoped>
.confirm-mask {
  position: fixed;
  inset: 0;
  background: rgba(10, 12, 16, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
}
.confirm-card {
  width: 420px;
  max-width: 92vw;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
  padding: 22px 24px;
  animation: fadeIn 0.16s ease;
}
.confirm-card.danger { border-top: 3px solid var(--red); }
.confirm-icon { color: var(--red); margin-bottom: 8px; }
.confirm-card h3 { margin: 0 0 8px; font-size: 16px; }
.confirm-msg { margin: 0; color: var(--text-2); line-height: 1.6; font-size: 13px; }
.confirm-input { margin-top: 14px; }
.confirm-input label { display: block; font-size: 12px; color: var(--text-2); margin-bottom: 6px; }
.confirm-input code {
  font-family: var(--font-mono);
  background: var(--danger-bg);
  color: var(--red);
  padding: 1px 6px;
  border-radius: 4px;
}
.confirm-input input { width: 100%; font-family: var(--font-mono); }
.confirm-actions {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
