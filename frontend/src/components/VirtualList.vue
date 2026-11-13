<template>
  <div class="vlist" ref="rootEl" @scroll="onScroll">
    <div class="vlist-spacer" :style="{ height: totalHeight + 'px' }"></div>
    <div class="vlist-window" :style="{ transform: `translateY(${offsetY}px)` }">
      <div
        v-for="idx in visibleIdx"
        :key="itemKey(items[idx - 1], idx - 1)"
        class="vlist-row"
        :style="{ height: rowHeight + 'px' }"
      >
        <slot :item="items[idx - 1]" :index="idx - 1" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'

// 固定行高虚拟滚动列表：十万级数据流畅滚动
const props = defineProps<{
  items: any[]
  rowHeight: number
  itemKey?: (item: any, idx: number) => string | number
  overscan?: number
}>()

const rootEl = ref<HTMLElement>()
const scrollTop = ref(0)
const viewportH = ref(400)

const totalHeight = computed(() => props.items.length * props.rowHeight)
const startIdx = computed(() => Math.max(0, Math.floor(scrollTop.value / props.rowHeight) - (props.overscan ?? 6)))
const endIdx = computed(() =>
  Math.min(props.items.length, Math.ceil((scrollTop.value + viewportH.value) / props.rowHeight) + (props.overscan ?? 6)),
)
const visibleIdx = computed(() => {
  const out: number[] = []
  for (let i = startIdx.value + 1; i <= endIdx.value; i++) out.push(i)
  return out
})
const offsetY = computed(() => startIdx.value * props.rowHeight)

function onScroll() {
  scrollTop.value = rootEl.value?.scrollTop ?? 0
}

function onResize() {
  viewportH.value = rootEl.value?.clientHeight ?? 400
}

onMounted(() => {
  onResize()
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => window.removeEventListener('resize', onResize))

function itemKey(item: any, idx: number): string | number {
  if (props.itemKey) return props.itemKey(item, idx)
  return idx
}

defineExpose({
  scrollToTop: () => {
    if (rootEl.value) rootEl.value.scrollTop = 0
  },
})
</script>

<style scoped>
.vlist {
  position: relative;
  height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
}
.vlist-spacer { pointer-events: none; }
.vlist-window {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  will-change: transform;
}
.vlist-row { display: flex; align-items: center; }
</style>
