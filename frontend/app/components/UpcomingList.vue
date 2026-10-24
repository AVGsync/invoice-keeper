<script setup lang="ts">
import type { Period } from '~/utils/periods'

const props = defineProps<{ items: Period[], canMark?: (period: Period) => boolean, loading?: boolean }>()

const shortMonths = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']
const monthOf = (due: string) => shortMonths[Number(due.slice(3, 5)) - 1] ?? ''
defineEmits<{ mark: [period: Period] }>()

const { visible, hasMore, sentinel } = useLazyList(() => props.items)
</script>

<template>
  <ul class="list">
    <!-- grey rows of the same size until the payments arrive -->
    <template v-if="loading">
      <li v-for="n in 3" :key="`s${n}`" class="row">
        <UiSkeleton width="38px" height="38px" />
        <UiSkeleton width="60px" height="10px" class="pad" />
        <span class="stack"><UiSkeleton width="150px" /><UiSkeleton width="100px" height="10px" /></span>
        <UiSkeleton width="100px" />
        <UiSkeleton width="90px" />
        <UiSkeleton width="30px" height="10px" />
        <UiSkeleton width="80px" height="20px" />
        <span />
      </li>
    </template>
    <li v-for="(item, i) in visible" :key="item.id" class="row rise" :style="{ '--i': i }">
      <UiDateTile :month="monthOf(item.due)" :day="item.due.slice(0, 2)" />
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
        <UiButton v-if="!item.paidAt && (!canMark || canMark(item))" @click="$emit('mark', item)">Оплачено</UiButton>
      </span>
    </li>
    <li v-if="hasMore" ref="sentinel" class="sentinel" />
  </ul>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pad {
  margin-left: 12px;
}

.sentinel {
  height: 1px;
}

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
  /* every column has its least width and a share of what's left, so the list fills the page evenly */
  grid-template-columns:
    42px minmax(80px, 0.8fr) minmax(190px, 2.2fr) minmax(130px, 1.4fr)
    minmax(110px, 1.2fr) minmax(56px, 0.6fr) minmax(150px, 1.5fr) 112px;
  column-gap: 12px;
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
