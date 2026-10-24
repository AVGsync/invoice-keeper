<script setup lang="ts">
import type { PeriodStatus } from '~/utils/demo'

const props = defineProps<{
  status: PeriodStatus | 'active'
  label?: string
}>()

const labels = {
  pending: 'Ожидает',
  paid: 'Оплачено',
  late: 'Оплачено с опозданием',
  overdue: 'Просрочено',
  active: 'Активен',
}
</script>

<template>
  <span class="status" :class="props.status">
    <span class="dot" />
    {{ label ?? labels[props.status] }}
  </span>
</template>

<style scoped>
.status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.pending,
.active {
  color: var(--ink-2);
  font-weight: 400;
}

.pending .dot,
.active .dot {
  background: none;
  border: 1px solid var(--ink-3);
}

.paid { color: var(--green); }
.late { color: var(--amber); }
.overdue { color: var(--red); }
</style>
