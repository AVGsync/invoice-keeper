<script setup lang="ts" generic="T extends string">
import { haptic } from '../telegram/webapp'

const props = defineProps<{
  modelValue: T
  options: { value: T; label: string }[]
}>()
const emit = defineEmits<{ 'update:modelValue': [T] }>()

function pick(v: T): void {
  if (v === props.modelValue) return
  haptic.selection()
  emit('update:modelValue', v)
}
</script>

<template>
  <div
    class="segmented"
    role="tablist"
    :style="{
      '--n': options.length,
      '--i': Math.max(0, options.findIndex((o) => o.value === modelValue)),
    }"
  >
    <button
      v-for="opt in options"
      :key="opt.value"
      class="seg"
      :class="{ active: opt.value === modelValue }"
      type="button"
      @click="pick(opt.value)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>

<style scoped>
.segmented {
  position: relative;
  display: flex;
  padding: 4px;
  gap: 0;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: var(--graphite);
}
/* sliding white pill */
.segmented::before {
  content: '';
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 4px;
  width: calc((100% - 8px) / var(--n));
  border-radius: var(--radius-pill);
  background: var(--snow);
  transform: translateX(calc(var(--i) * 100%));
  transition: transform 0.36s var(--ease-drawer);
  pointer-events: none;
}
.seg {
  position: relative;
  z-index: 1;
  flex: 1;
  min-width: 0;
  min-height: 40px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--ash-text);
  font-size: var(--fs-small);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: color var(--dur) var(--ease);
}
.seg.active {
  color: var(--ink);
}
.seg:active:not(.active) {
  color: var(--snow);
}
</style>
