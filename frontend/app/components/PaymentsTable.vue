<script setup lang="ts">
import type { CurrencyTotal, Period } from '~/utils/demo'

withDefaults(defineProps<{ periods: Period[], totals: CurrencyTotal[], filtered?: boolean, month?: string }>(), { month: 'октябрь' })
// every demo payment (the `periods` prop only has the filtered ones).
// its index gives each row a stable view-transition-name
const { periods: allPeriods, enabled: demoOn } = useDemo()
defineEmits<{ reset: [] }>()

// title of the row whose "mark paid" popover is open
const open = ref<string>()

// paid rows only open (to fix the date or unmark) in demo mode
function toggle(period: Period) {
  if (period.paidAt && !demoOn.value) return
  open.value = open.value === period.title ? undefined : period.title
}
</script>

<template>
  <div class="card table">
    <div class="row head">
      <span class="mono muted">№</span>
      <span>Название</span>
      <span>Ответственный</span>
      <span>Срок</span>
      <span class="right">Сумма</span>
      <span class="currency">Валюта</span>
      <span>Статус</span>
      <span>Дата оплаты</span>
      <span class="right">Просрочка</span>
    </div>

    <div
      v-for="(period, i) in periods"
      :key="period.title"
      class="row"
      :class="{ overdue: period.status === 'overdue', clickable: !period.paidAt || demoOn, 'has-popover': open === period.title }"
      :style="{ viewTransitionName: `row-${allPeriods.indexOf(period)}` }"
      @click="toggle(period)"
    >
      <span class="mono muted small">{{ String(i + 1).padStart(2, '0') }}</span>
      <span>
        <span class="title">{{ period.title }}</span>
        <span class="small muted">{{ period.frequency }}</span>
      </span>
      <PersonCell :person="period.who" />
      <span>
        <span class="mono">{{ period.due }}</span>
        <span
          v-if="period.relative"
          class="small"
          :class="period.status === 'overdue' ? 'red' : period.relative === 'завтра' ? '' : 'muted'"
        >{{ period.relative }}</span>
      </span>
      <span class="mono strong right">{{ period.amount }}</span>
      <span class="mono small muted currency">{{ period.currency }}</span>
      <UiStatus :status="period.status" />
      <span class="mono" :class="{ muted: !period.paidAt }">{{ period.paidAt ?? '—' }}</span>
      <span class="mono right" :class="period.status === 'overdue' ? 'red strong' : period.status === 'late' ? 'amber strong' : 'muted'">
        {{ period.overdue ?? '—' }}
      </span>

      <Transition name="menu">
        <MarkPaidPopover v-if="open === period.title" :period="period" class="popover" @close="open = undefined" />
      </Transition>
    </div>

    <!-- nothing matched: say so and offer a way out -->
    <UiState v-if="!periods.length" class="empty">
      {{ filtered ? 'Под эти фильтры нет ни одного платежа.' : 'Платежей пока нет.' }}
      <template v-if="filtered">
        <br>
        <button type="button" class="reset" @click="$emit('reset')">Сбросить фильтры</button>
      </template>
    </UiState>

    <div v-else class="foot">
      <span>
        <span class="title">Итого за {{ month }}</span>
        <span class="small muted">по каждой валюте отдельно</span>
      </span>
      <span v-for="total in totals" :key="total.code" class="foot-total">
        <span class="mono small soft strong">{{ total.code }}</span>
        <span>
          <span class="mono strong">{{ total.total }}</span>
          <span class="mono small muted">оплачено {{ total.paid }}</span>
        </span>
      </span>
    </div>
  </div>
</template>

<style scoped>
/* filter changes: the table resizes, rows slide to their new place (useViewTransition) */
.table {
  view-transition-name: payments-table;
  position: relative;
  font-size: 13px;
}

/* thin blue line next to the row numbers, like a ledger margin */
.table::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 63px;
  left: 44px;
  width: 1px;
  background: color-mix(in srgb, var(--accent) 35%, transparent);
}

.row {
  position: relative;
  display: grid;
  grid-template-columns: 36px 224px 150px 110px 140px 90px 190px 90px 74px;
  align-items: center;
  height: 56px;
  padding: 0 20px;
  border-top: 1px solid var(--line);
}

.row.head {
  height: 40px;
  border-top: 0;
  border-radius: 10px 10px 0 0;
  background: var(--subtle);
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-2);
}

/* every row is its own layer (view-transition-name), so the one with the popover
   has to sit above the rows and footer below it */
.row.has-popover {
  z-index: 3;
}

.row.clickable {
  cursor: pointer;
}

.row:not(.head):hover {
  background: color-mix(in srgb, var(--subtle) 55%, transparent);
}

.row.overdue {
  box-shadow: inset 3px 0 var(--red);
}

.title {
  display: block;
  font-size: 14px;
  font-weight: 500;
}

.small {
  display: block;
  font-size: 12px;
}

.strong {
  font-weight: 500;
}

.currency {
  padding-left: 20px;
}

.red { color: var(--red); }
.amber { color: var(--amber); }

.popover {
  position: absolute;
  top: calc(100% + 6px);
  right: 16px;
  z-index: 2;
  cursor: default;
}

.reset {
  margin-top: 10px;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
}

.foot {
  view-transition-name: payments-foot;
  display: flex;
  align-items: center;
  height: 63px;
  padding: 0 20px;
  border-top: 1px solid var(--line);
  border-radius: 0 0 10px 10px;
  background: var(--subtle);
}

.foot > span:first-child {
  width: 180px;
  font-size: 13px;
}

.foot > span:first-child .title {
  font-size: 13px;
}

.foot-total {
  display: flex;
  gap: 18px;
  width: 220px;
}
</style>
