<script setup lang="ts">
import StatusBadge from '../StatusBadge.vue'
import { fromAssistant } from '../../lib/status'
import type { OccurrenceListBlock } from '../../api/types'

defineProps<{ block: OccurrenceListBlock }>()
defineEmits<{ open: [string] }>()
</script>

<template>
  <div class="block">
    <div v-if="block.title" class="block-title">{{ block.title }}</div>
    <button
      v-for="item in block.items"
      :key="item.occurrence_id"
      type="button"
      class="brow row-active"
      @click="$emit('open', item.occurrence_id)"
    >
      <div class="left">
        <div class="t">{{ item.title }}</div>
        <div class="m">
          <StatusBadge :status="fromAssistant(item.status)" />
        </div>
      </div>
      <span class="num mono-amount">{{ item.display }}</span>
    </button>
  </div>
</template>

<style scoped>
.block {
  background: var(--c-section);
  border-radius: var(--radius);
  overflow: hidden;
  margin: 8px 0;
}
.block-title {
  padding: 10px 14px 4px;
  color: var(--c-hint);
  font-size: 13px;
}
.brow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--c-text);
  text-align: left;
  padding: 10px 14px;
  border-top: 1px solid var(--c-secondary-bg);
}
.left {
  min-width: 0;
  flex: 1;
}
.t {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.m {
  margin-top: 2px;
}
</style>
