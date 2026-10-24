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
  /* правый padding убран: WebKit не прокручивает padding-inline-end — его
     заменяет хвостовой спейсер ::after */
  padding: 2px 0 2px var(--gutter);
  scroll-padding-inline: var(--gutter);
  scrollbar-width: none;
  overscroll-behavior-x: contain;
}
.chips::-webkit-scrollbar {
  display: none;
}
/* хвостовой отступ — flex-элемент, который WebKit честно прокручивает (gap 8 + 8 = gutter 16) */
.chips::after {
  content: '';
  flex: 0 0 max(1px, calc(var(--gutter) - 8px));
}
.chip {
  position: relative;
  flex: 0 0 auto;
  min-height: 36px;
  padding: 8px 14px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--ash-text);
  font-size: var(--fs-small);
  font-weight: 500;
  white-space: nowrap;
  transition:
    background-color var(--dur-focus) var(--ease-press),
    border-color var(--dur-focus) var(--ease-press),
    color var(--dur-focus) var(--ease-press);
}
/* 44px hit area, 36px visual */
.chip::after {
  content: '';
  position: absolute;
  inset: -4px 0;
}
.chip.active {
  border-color: var(--snow);
  background: var(--snow);
  color: var(--ink);
}
.chip:active:not(.active) {
  background: var(--press);
}
</style>
