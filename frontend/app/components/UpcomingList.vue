<script setup lang="ts">
import type { Period } from '~/utils/demo'

defineProps<{ items: Period[], soon?: boolean }>()
defineEmits<{ mark: [period: Period] }>()
</script>

<template>
  <ul class="list">
    <li v-for="(item, i) in items" :key="item.title" class="row rise" :style="{ '--i': i }">
      <UiDateTile month="окт" :day="item.due.slice(0, 2)" />
      <span class="relative" :class="{ near: item.relative === 'завтра' }">{{ item.relative }}</span>
      <span>
        <span class="title">{{ item.title }}</span>
        <span class="meta">{{ item.meta }}</span>
      </span>
      <PersonCell :person="item.who" />
      <span class="amount mono">{{ item.amount }}</span>
      <span class="currency mono">{{ item.currency }}</span>
      <span>
        <UiStatus v-if="item.paidAt" status="paid" :label="`Оплачено ${item.paidAt.slice(0, 5)}`" />
        <UiStatus v-else :status="item.status" />
      </span>
      <span class="action">
        <UiButton v-if="!item.paidAt" :soon="soon" @click="$emit('mark', item)">Оплачено</UiButton>
      </span>
    </li>
  </ul>
</template>

<style scoped>
/* no box: hairlines above, between and below the rows */
.list {
  margin: 0 -12px;
  padding: 0;
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
  list-style: none;
}

.row {
  display: grid;
  grid-template-columns: 42px 98px 230px 150px 150px 100px 1fr 112px;
  align-items: center;
  height: 60px;
  padding: 0 12px;
  transition: background-color 0.2s ease;
}

.row:hover {
  background: var(--subtle);
}

.row + .row {
  border-top: 1px solid var(--line);
}

.relative {
  padding-left: 12px;
  font-size: 12px;
  color: var(--ink-3);
}

.relative.near {
  color: var(--ink);
  font-weight: 500;
}

.title {
  display: block;
  font-weight: 500;
}

.meta {
  font-size: 12px;
  color: var(--ink-3);
}

.amount {
  font-weight: 500;
  text-align: right;
}

.currency {
  padding-left: 16px;
  font-size: 12px;
  color: var(--ink-3);
}

.action {
  text-align: right;
}

.action .btn {
  width: 112px;
}
</style>
