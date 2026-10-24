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
  <div class="segmented" role="tablist">
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
  display: flex;
  background: var(--c-secondary-bg);
  border-radius: 10px;
  padding: 2px;
  gap: 2px;
}
.seg {
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--c-text);
  padding: 8px 10px;
  border-radius: 8px;
  font-weight: 500;
  font-size: 14px;
}
.seg.active {
  background: var(--c-primary);
  color: var(--c-primary-text);
}
.seg:active {
  opacity: 0.85;
}
</style>
