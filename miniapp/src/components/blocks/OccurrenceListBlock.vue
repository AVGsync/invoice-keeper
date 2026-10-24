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
  background: var(--graphite);
  border: 1px solid var(--line);
  border-radius: 20px;
  overflow: hidden;
  margin: 0;
}
.block-title {
  padding: var(--sp-4) var(--sp-5) var(--sp-2);
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--fog);
  line-height: 1.2;
}
.brow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--bone);
  text-align: left;
  padding: 14px var(--sp-5);
  border-top: 1px solid var(--c-line);
}
.left {
  min-width: 0;
  flex: 1;
}
.t {
  font-size: var(--fs-body);
  font-weight: 400;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.m {
  margin-top: 4px;
}
</style>
