<script setup lang="ts">
import type { PeopleListBlock } from '../../api/types'

defineProps<{ block: PeopleListBlock }>()
</script>

<template>
  <div class="block">
    <div v-if="block.title" class="block-title">{{ block.title }}</div>
    <div v-for="p in block.items" :key="p.user_id" class="prow">
      <div class="name">{{ p.full_name }}</div>
      <div class="hint">
        <span v-if="p.persona">{{ p.persona }}</span>
        <span v-if="p.active_payments != null"> · платежей: {{ p.active_payments }}</span>
      </div>
    </div>
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
.prow {
  padding: 10px 14px;
  border-top: 1px solid var(--c-secondary-bg);
}
.name {
  font-weight: 500;
}
</style>
