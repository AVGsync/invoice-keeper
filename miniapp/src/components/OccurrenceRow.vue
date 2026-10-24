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
        <span class="when"
          ><svg v-if="occ.receipts_count" class="clip" viewBox="0 0 24 24" aria-label="Есть чеки" role="img"
            ><path d="M20 11.5l-8.2 8.2a5 5 0 0 1-7-7l8.5-8.5a3.3 3.3 0 0 1 4.7 4.7L9.6 17.2a1.7 1.7 0 0 1-2.4-2.4l7.6-7.6" /></svg
          >{{ rel
          }}<template v-if="showAssignee && occ.assignee_name"> · {{ occ.assignee_name }}</template></span
        >
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
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  color: var(--muted);
  font-size: var(--fs-small);
  margin-top: 4px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}
.clip {
  width: 14px;
  height: 14px;
  margin-right: 4px;
  vertical-align: -2px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.when {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.occ-row :deep(.num) {
  font-size: var(--fs-body-l);
  color: var(--bone);
}
</style>
