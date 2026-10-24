<script setup lang="ts">
defineProps<{
  label: string
  hint?: string
}>()

// UiInput / UiDropdown inside pick this id up, so clicking the label focuses them.
// wrapping everything in <label> instead would tie it to the first button too (e.g. «Сгенерировать»)
const id = useId()
provide('fieldId', id)
</script>

<template>
  <div class="field">
    <div class="top">
      <label class="label" :for="id">{{ label }}</label>
      <slot name="aside" />
    </div>
    <slot />
    <span v-if="hint" class="hint">{{ hint }}</span>
  </div>
</template>

<style scoped>
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.top {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-2);
  transition: color 0.2s ease;
}

.hint {
  font-size: 12px;
  color: var(--ink-3);
}

/* the label of the field being typed in gets brighter */
.field:focus-within .label {
  color: var(--ink);
}
</style>
