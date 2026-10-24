<script setup lang="ts">
import { computed } from 'vue'
import { statusInfo } from '../lib/status'
import type { OccStatus } from '../api/types'

const props = defineProps<{ status: OccStatus; paidLate?: boolean }>()
const info = computed(() => statusInfo({ status: props.status, paid_late: props.paidLate }))
</script>

<template>
  <span class="badge" :class="['badge--' + status, { late: paidLate }]">{{ info.label }}</span>
</template>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 9px;
  border-radius: var(--radius-pill);
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
  white-space: nowrap;
  border: 1px solid transparent;
}
.badge--overdue {
  background: var(--red-bg);
  color: var(--red-ink);
}
.badge--pending {
  background: var(--accent-bg);
  color: var(--accent-ink);
}
.badge--paid {
  border: 0;
  padding-left: 0;
  color: var(--green-ink);
}
/* оплачено с опозданием — янтарный акцент */
.badge--paid.late {
  color: var(--amber-ink);
}
.badge--paid::before {
  content: '';
  width: 8px;
  height: 4px;
  border-left: 1.5px solid currentColor;
  border-bottom: 1.5px solid currentColor;
  transform: translateY(-2px) rotate(-45deg);
}
.badge--ended {
  border: 1px dashed var(--slate);
  color: var(--muted);
}
</style>
