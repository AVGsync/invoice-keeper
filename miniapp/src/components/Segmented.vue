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
  border: 1px solid var(--ash);
  border-radius: 0;
  padding: 0;
  gap: 0;
  background: transparent;
}
.segmented::before {
  content: '';
  position: absolute;
  top: -1px;
  bottom: -1px;
  left: -1px;
  width: calc((100% + 2px) / var(--n));
  border: 1px solid var(--bone);
  transform: translateX(calc(var(--i) * 100%));
  transition: transform var(--dur) var(--ease);
  pointer-events: none;
}
.seg {
  flex: 1;
  min-width: 0;
  min-height: 44px;
  padding: 0 8px;
  border: 0;
  background: transparent;
  color: var(--fog);
  font-size: var(--fs-caption);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  border-radius: 0;
  transition: color var(--dur) var(--ease);
}
@media (max-width: 374px) {
  .seg {
    letter-spacing: 0.02em;
  }
}
.seg.active {
  color: var(--bone);
  background: transparent;
}
.seg:active {
  opacity: 0.6;
}
</style>
