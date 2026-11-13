<template>
  <span class="tag type-badge" :style="{ background: `color-mix(in srgb, ${color} 14%, transparent)`, color }">
    {{ meta.label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { typeMeta } from '../lib/format'

const props = defineProps<{ type: string }>()
const meta = computed(() => typeMeta(props.type))
const color = computed(() => meta.value.color)
</script>

<style scoped>
.type-badge { font-size: 10px; padding: 1px 7px; letter-spacing: 0.4px; }
</style>
