<!-- multiple choice filter: every chip turns on/off, at least one stays on -->
<script setup lang="ts">
export interface ChipOption {
  value: string
  label: string
  // shown before the label, e.g. ₽ for RUB
  symbol?: string
}

defineProps<{ options: ChipOption[] }>()

const model = defineModel<string[]>({ default: () => [] })

function toggle(value: string) {
  const on = model.value.includes(value)
  // switching off the last chip would just leave an empty list
  if (on && model.value.length === 1) return
  model.value = on ? model.value.filter(item => item !== value) : [...model.value, value]
}
</script>

<template>
  <div class="chips" role="group">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="chip mono"
      :class="{ on: model.includes(option.value) }"
      :aria-pressed="model.includes(option.value)"
      @click="toggle(option.value)"
    >
      <span v-if="option.symbol" class="symbol">{{ option.symbol }}</span>{{ option.label }}
    </button>
  </div>
</template>

<style scoped>
.chips {
  display: flex;
  gap: 6px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px;
  background: none;
  border: 1px solid var(--line);
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
  transition: background 0.14s ease, color 0.14s ease, border-color 0.14s ease;
}

.chip.on {
  background: var(--subtle);
  border-color: var(--line-strong);
  color: var(--ink);
}

.symbol {
  font-family: var(--sans);
  font-size: 13px;
}
</style>
