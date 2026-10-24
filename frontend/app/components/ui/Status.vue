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
/* pill with a faint wash of its colour, the only coloured thing in a row */
.status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 9px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  /* in a table cell (grid) it keeps its own width instead of stretching */
  justify-self: start;
  width: max-content;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.pending,
.active {
  background: var(--subtle);
  color: var(--ink-2);
}

.pending .dot,
.active .dot {
  background: var(--ink-3);
}

.paid { background: var(--green-bg); color: var(--green); }
.late { background: var(--amber-bg); color: var(--amber); }
.overdue { background: var(--red-bg); color: var(--red); }

.overdue .dot {
  animation: ping 2s ease-out infinite;
}

@keyframes ping {
  0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--red) 55%, transparent); }
  70%, 100% { box-shadow: 0 0 0 5px transparent; }
}
</style>
