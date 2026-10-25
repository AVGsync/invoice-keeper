<script setup lang="ts">
import type { Period } from '~/utils/periods'

const { withTransition } = useViewTransition()

const payments = usePayments()
onMounted(payments.load)

// 'YYYY-MM' shown in the grid, starts on the current month
const thisMonth = computed(() => payments.today.value.slice(0, 7))
const month = ref(thisMonth.value)
const shown = computed(() => parseMonthKey(month.value))

// kept as an id: after a reload the list has new objects for the same payments
const selectedId = ref<string>()
const selected = computed<Period | undefined>({
  get: () => payments.periods.value.find(period => period.id === selectedId.value),
  set: (period) => { selectedId.value = period?.id },
})

// opens on the nearest unpaid payment, once the list is there
watchEffect(() => {
  if (selectedId.value) return
  const next = payments.periods.value.find(period => !period.paidAt && period.status !== 'ended'
    && fromRu(period.due) >= payments.today.value && fromRu(period.due).startsWith(month.value))
  selectedId.value = next?.id
})

// another month: the picked day isn't on screen anymore, so the side panel goes too
const monthModel = computed({
  get: () => month.value,
  set: value => withTransition(() => {
    month.value = value
    if (selected.value && !fromRu(selected.value.due).startsWith(value)) selected.value = undefined
  }),
})

function goToday() {
  monthModel.value = thisMonth.value
}

function mark(period: Period) {
  payments.markPaid(period, payments.today.value).catch(() => {})
}

const selectedIso = computed(() => (selected.value ? fromRu(selected.value.due) : ''))
const day = computed(() => Number(selectedIso.value.slice(8, 10)))
const weekday = computed(() => (selectedIso.value ? new Date(selectedIso.value).toLocaleDateString('ru-RU', { weekday: 'long' }) : ''))

// «Итоги октября» for the shown month
const monthPeriods = computed(() => payments.periods.value.filter(period => fromRu(period.due).startsWith(month.value)))
const monthTotals = computed(() => totalsOn(lastDayOf(month.value), monthPeriods.value))

// the badge under the totals counts what's overdue right now
const overdueNow = computed(() => payments.periods.value.filter(period => period.status === 'overdue'))
// the selected day's other payments, listed under the selected one
const sameDay = computed(() => payments.periods.value.filter(period => period.due === selected.value?.due && period.id !== selected.value?.id))
const kicker = computed(() => [weekday.value, selected.value?.relative].filter(Boolean).join(' · '))

// on a phone the details are under the grid: bring them into view after a tap on another day
// (the automatic pick on load and on a month change has no previous value, so it doesn't scroll)
watch(selectedId, async (id, before) => {
  if (!id || !before || !window.matchMedia('(max-width: 1240px)').matches) return
  await nextTick()
  document.querySelector('.side')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
})
</script>

<template>
  <div>
    <AppPageHeader title="Календарь" />

    <div class="toolbar">
      <UiMonthPicker v-model="monthModel" variant="title" :current="thisMonth" />
      <!-- back to the current month -->
      <UiButton class="today" aria-label="Сегодня" title="К сегодняшнему месяцу" @click="goToday"><svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
          <!-- drawn around the middle of its box, so it sits centred in the round button -->
          <path d="M8 2.5A5.5 5.5 0 1 1 3.24 5.25M9.9 0.9L8 2.5L9.9 4.1" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        </svg></UiButton>
      <span class="legend">
        <UiStatus status="pending" label="ожидает" />
        <UiStatus status="paid" label="оплачено" />
        <UiStatus status="late" label="с опозданием" />
        <UiStatus status="overdue" label="просрочено" />
      </span>
    </div>

    <div class="layout">
      <CalendarGrid
        v-model:selected="selected"
        :year="shown.year"
        :month="shown.month"
        :today="month === thisMonth ? Number(payments.today.value.slice(8)) : 0"
        :periods="payments.periods.value"
      />

      <div class="side">
        <!-- a new key for every payment, so its lines come up again -->
        <DayPanel
          v-if="selected"
          :key="selected.id"
          :kicker="kicker"
          :title="`${day} ${monthNamesGenitive[shown.month]}`"
          :period="selected"
          :others="sameDay"
          :can-mark="payments.canMark(selected)"
          @mark="mark(selected)"
          @pick="period => (selected = period)"
        />
        <MonthTotals :title="`Итоги ${monthNamesGenitive[shown.month]}`" :totals="monthTotals" :loading="!payments.ready.value" />
        <div v-if="overdueNow.length" class="note">
          <UiStatus status="overdue" :label="`${overdueNow.length} просрочен${overdueNow.length > 1 ? 'о' : ''}`" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
}

.today {
  display: grid;
  place-items: center;
  height: 32px;
  width: 32px;
  padding: 0;
  color: var(--ink-2);
}

.today:hover {
  color: var(--ink);
}

.legend {
  display: flex;
  gap: 18px;
  margin-left: 14px;
}

/* legend shows just the coloured dot and word, without the badge fill */
.legend :deep(.status) {
  height: auto;
  padding: 0;
  background: none;
  font-weight: 400;
}

.layout {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 16px;
}

/* a column of its own next to the grid, split off by a hairline like a table's */
.side {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-left: 20px;
  border-left: 1px solid var(--line);
}

.note {
  view-transition-name: margin-note;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

/* phones */
@media (max-width: 1240px) {
  .toolbar {
    flex-wrap: wrap;
    margin-bottom: 14px;
  }

  .legend {
    flex: 1 1 100%;
    flex-wrap: wrap;
    gap: 6px 14px;
    margin-left: 0;
  }

  .layout {
    grid-template-columns: minmax(0, 1fr);
  }

  /* the day's details go under the grid */
  .side {
    padding-left: 0;
    border-left: 0;
  }
}
</style>
