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
  <div class="chips">
    <button
      v-for="opt in options"
      :key="opt.value"
      type="button"
      class="chip"
      :class="{ active: opt.value === modelValue }"
      @click="pick(opt.value)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>

<style scoped>
.chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding: 2px 0;
  scrollbar-width: none;
}
.chips::-webkit-scrollbar {
  display: none;
}
.chip {
  flex: 0 0 auto;
  border: 1px solid var(--c-secondary-bg);
  background: var(--c-secondary-bg);
  color: var(--c-text);
  padding: 7px 14px;
  border-radius: 999px;
  font-size: 14px;
  white-space: nowrap;
}
.chip.active {
  background: var(--c-primary);
  color: var(--c-primary-text);
  border-color: var(--c-primary);
}
.chip:active {
  opacity: 0.85;
}
</style>
