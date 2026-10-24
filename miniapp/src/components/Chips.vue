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
  margin: 0 calc(-1 * var(--gutter));
  padding: 2px var(--gutter);
  scroll-padding-inline: var(--gutter);
  scrollbar-width: none;
}
.chips::-webkit-scrollbar {
  display: none;
}
.chip {
  position: relative;
  flex: 0 0 auto;
  min-height: 36px;
  padding: 8px 14px;
  border: 1px solid var(--ash);
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--fog);
  font-size: var(--fs-small);
  letter-spacing: 0.01em;
  white-space: nowrap;
  transition:
    border-color var(--dur-focus) var(--ease-press),
    color var(--dur-focus) var(--ease-press);
}
.chip::after {
  content: '';
  position: absolute;
  inset: -4px 0;
}
.chip.active {
  border-color: var(--bone);
  color: var(--bone);
  background: transparent;
}
.chip:active {
  background: var(--press);
}
</style>
