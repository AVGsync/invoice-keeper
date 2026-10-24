<script setup lang="ts">
defineProps<{ label?: string; value?: string; chevron?: boolean; clickable?: boolean }>()
</script>

<template>
  <div class="cell" :class="{ 'row-active': clickable }">
    <div class="cell-main">
      <div v-if="label" class="cell-label">{{ label }}</div>
      <div class="cell-content"><slot>{{ value }}</slot></div>
    </div>
    <div v-if="$slots.right || chevron" class="cell-right">
      <slot name="right" />
      <span v-if="chevron" class="chevron">›</span>
    </div>
  </div>
</template>

<style scoped>
.cell {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.cell-main {
  min-width: 0;
  flex: 1;
}
.cell-label {
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--fog);
  line-height: 1.2;
  margin-bottom: 4px;
}
.cell-content {
  font-size: var(--fs-body-l);
  word-break: break-word;
}
.cell-right {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--fog);
  flex-shrink: 0;
}
.chevron {
  color: var(--fog);
  font-size: 20px;
}
</style>
