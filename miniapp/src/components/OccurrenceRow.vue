<script setup lang="ts">
import { computed } from 'vue'
import Money from './Money.vue'
import StatusBadge from './StatusBadge.vue'
import { relativeDay, formatShort, today } from '../lib/dates'
import type { PaymentResponse } from '../api/types'

const props = defineProps<{
  occ: PaymentResponse
  showAssignee?: boolean
  showStatus?: boolean
}>()
defineEmits<{ open: [string] }>()

// «просрочено/через N дн.» имеет смысл только для неоплаченных периодов.
const rel = computed(() => {
  const o = props.occ
  if (o.status === 'paid') return o.paid_date ? 'оплачено ' + formatShort(o.paid_date) : 'оплачено'
  if (o.status === 'ended') return 'остановлен'
  return relativeDay(o.due_date, today())
})
</script>

<template>
  <button class="occ-row row-active" type="button" @click="$emit('open', occ.id)">
    <div class="left">
      <div class="desc">{{ occ.description }}</div>
      <div class="meta">
        <span>{{ rel }}</span>
        <span v-if="showAssignee && occ.assignee_name"> · {{ occ.assignee_name }}</span>
        <StatusBadge v-if="showStatus" :status="occ.status" :paid-late="occ.paid_late" />
      </div>
    </div>
    <Money :amount="occ.amount" :currency="occ.currency" />
  </button>
</template>

<style scoped>
.occ-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  text-align: left;
  border: 0;
  background: transparent;
  color: var(--bone);
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.left {
  min-width: 0;
  flex: 1;
}
.desc {
  font-size: var(--fs-body-l);
  font-weight: 400;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  color: var(--fog);
  font-size: var(--fs-small);
  margin-top: 4px;
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.meta::after {
  content: '';
  flex-basis: 100%;
  order: 1;
}
.meta > .badge {
  order: 2;
}
.meta > span {
  white-space: nowrap;
}
.occ-row :deep(.num) {
  font-size: var(--fs-body-l);
  color: var(--bone);
}
</style>
