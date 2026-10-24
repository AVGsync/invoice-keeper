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
  background: var(--obsidian);
  border: 1px solid var(--c-line);
  border-radius: var(--radius);
  overflow: hidden;
  margin: 0;
}
.block-title {
  padding: var(--sp-4) var(--sp-5) var(--sp-2);
  font-size: var(--fs-caption);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--fog);
  line-height: 1.2;
}
.prow {
  padding: 14px var(--sp-5);
  border-top: 1px solid var(--c-line);
}
.name {
  font-size: var(--fs-body);
  font-weight: 400;
}
</style>
