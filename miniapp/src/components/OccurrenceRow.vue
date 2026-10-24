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
  color: var(--c-text);
  padding: 12px 14px;
  border-bottom: 1px solid var(--c-secondary-bg);
}
.occ-row:last-child {
  border-bottom: none;
}
.left {
  min-width: 0;
  flex: 1;
}
.desc {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  color: var(--c-hint);
  font-size: 13px;
  margin-top: 2px;
  display: flex;
  gap: 6px;
  align-items: center;
  flex-wrap: wrap;
}
</style>
