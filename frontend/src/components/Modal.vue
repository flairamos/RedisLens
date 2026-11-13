<template>
  <Teleport to="body">
    <div v-if="visible" class="modal-mask" @click.self="onMaskClick">
      <div class="modal-card" :style="{ width }" role="dialog">
        <header class="modal-head">
          <h3>{{ title }}</h3>
          <button class="btn ghost sm" @click="close" aria-label="关闭">
            <Icon name="x" :size="16" />
          </button>
        </header>
        <div class="modal-body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="modal-foot">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import Icon from './Icon.vue'

defineProps<{
  visible: boolean
  title: string
  width?: string
  closableMask?: boolean
}>()
const emit = defineEmits<{ (e: 'close'): void }>()

function onMaskClick() {
  emit('close')
}
function close() {
  emit('close')
}
</script>

<style scoped>
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(10, 12, 16, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal-card {
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
  max-width: 92vw;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  animation: fadeIn 0.16s ease;
  min-width: 380px;
}
.modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}
.modal-head h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.modal-body {
  padding: 16px 18px;
  overflow: auto;
}
.modal-foot {
  padding: 12px 18px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
