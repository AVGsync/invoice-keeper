<script setup lang="ts">
import type { Period } from '~/utils/demo'

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

// the margin note follows what's actually overdue
const overdueNow = computed(() => payments.periods.value.filter(period => period.status === 'overdue'))
// the selected day's other payments, listed under the selected one
const sameDay = computed(() => payments.periods.value.filter(period => period.due === selected.value?.due && period.id !== selected.value?.id))
const kicker = computed(() => [weekday.value, selected.value?.relative].filter(Boolean).join(' · '))
</script>

<template>
  <div>
    <AppPageHeader title="Календарь" subtitle="Сроки оплат по дням — с отметкой статуса" />

    <div class="toolbar">
      <UiMonthPicker v-model="monthModel" variant="title" :current="thisMonth" />
      <UiButton class="today" @click="goToday">Сегодня</UiButton>
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
        <DayPanel
          v-if="selected"
          :kicker="kicker"
          :title="`${day} ${monthNamesGenitive[shown.month]}`"
          :period="selected"
          :others="sameDay"
          :can-mark="payments.canMark(selected)"
          @mark="mark(selected)"
          @pick="period => (selected = period)"
        />
        <MonthTotals :title="`Итоги ${monthNamesGenitive[shown.month]}`" :totals="monthTotals" />
        <div v-if="overdueNow.length" class="note">
          <p class="italic margin">
            Отметка на полях:<br>{{ overdueNow[0]!.title.toLowerCase() }} за {{ Number(overdueNow[0]!.due.slice(0, 2)) }}-е ещё не оплачена.
          </p>
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
  margin: -10px 0 18px;
}

.today {
  height: 32px;
  width: 84px;
  padding: 0;
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

.margin {
  font-size: 13px;
  line-height: 18px;
  color: var(--ink-2);
}
</style>
