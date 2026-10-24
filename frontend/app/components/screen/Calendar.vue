<script setup lang="ts">
import type { Period } from '~/utils/demo'

const { withTransition } = useViewTransition()

// 'YYYY-MM' shown in the grid, starts on the demo's current month
const demoMonth = demoToday.slice(0, 7)
const month = ref(demoMonth)
const shown = computed(() => parseMonthKey(month.value))

const demo = useDemo()
const selected = ref<Period | undefined>(demo.periods.value.find(period => period.title === 'Сервер VPS'))

// another month: the picked day isn't on screen anymore, so the side panel goes too
const monthModel = computed({
  get: () => month.value,
  set: value => withTransition(() => {
    month.value = value
    if (selected.value && !fromRu(selected.value.due).startsWith(value)) selected.value = undefined
  }),
})

function goToday() {
  monthModel.value = demoMonth
}

const selectedIso = computed(() => (selected.value ? fromRu(selected.value.due) : ''))
const day = computed(() => Number(selectedIso.value.slice(8, 10)))
const weekday = computed(() => (selectedIso.value ? new Date(selectedIso.value).toLocaleDateString('ru-RU', { weekday: 'long' }) : ''))

// «Итоги октября» for the shown month
const monthPeriods = computed(() => demo.periods.value.filter(period => fromRu(period.due).startsWith(month.value)))
const monthTotals = computed(() => totalsOn(lastDayOf(month.value), monthPeriods.value))

// the margin note follows what's actually overdue
const overdueNow = computed(() => demo.periods.value.filter(period => period.status === 'overdue'))
const kicker = computed(() => [weekday.value, selected.value?.relative].filter(Boolean).join(' · '))
</script>

<template>
  <div>
    <AppPageHeader title="Календарь" subtitle="Сроки оплат по дням — с отметкой статуса" />

    <div class="toolbar">
      <UiMonthPicker v-model="monthModel" variant="title" :current="demoMonth" />
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
        :today="month === demoMonth ? Number(demoToday.slice(8)) : 0"
        :periods="demo.periods.value"
      />

      <div class="side">
        <DayPanel
          v-if="selected"
          :kicker="kicker"
          :title="`${day} ${monthNamesGenitive[shown.month]}`"
          :period="selected"
          :soon="!demo.enabled.value"
          @mark="demo.markPaid(selected, demoToday)"
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

/* legend shows just the dot and the word, without the badge fill */
.legend :deep(.status) {
  height: auto;
  padding: 0;
  background: none;
  font-weight: 400;
  color: var(--ink-2);
}

.layout {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 16px;
}

.side {
  display: flex;
  flex-direction: column;
  gap: 16px;
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
