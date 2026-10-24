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
  padding: 12px 14px;
  border-bottom: 1px solid var(--c-secondary-bg);
}
.cell:last-child {
  border-bottom: none;
}
.cell-main {
  min-width: 0;
  flex: 1;
}
.cell-label {
  color: var(--c-hint);
  font-size: 13px;
  margin-bottom: 2px;
}
.cell-content {
  word-break: break-word;
}
.cell-right {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--c-hint);
  flex-shrink: 0;
}
.chevron {
  color: var(--c-hint);
  font-size: 20px;
}
</style>
