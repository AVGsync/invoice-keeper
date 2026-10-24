<script setup lang="ts">
import type { Period } from '~/utils/demo'

const { withTransition } = useViewTransition()

// 'YYYY-MM' shown in the grid, starts on the demo's current month
const demoMonth = demoToday.slice(0, 7)
const month = ref(demoMonth)
const shown = computed(() => parseMonthKey(month.value))

const selected = ref<Period | undefined>(periods.find(period => period.title === 'Сервер VPS'))

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
const monthTotals = computed(() => totalsOn(lastDayOf(month.value), periods.filter(period => fromRu(period.due).startsWith(month.value))))
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
        :periods="periods"
      />

      <div class="side">
        <DayPanel v-if="selected" :kicker="kicker" :title="`${day} ${monthNamesGenitive[shown.month]}`" :period="selected" />
        <MonthTotals :title="`Итоги ${monthNamesGenitive[shown.month]}`" :totals="monthTotals" />
        <div class="note">
          <p class="italic margin">Отметка на полях:<br>аренда за 18-е ещё не оплачена.</p>
          <UiStamp color="red" large class="stamp">1 ПРОСРОЧЕН</UiStamp>
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

.legend :deep(.status) {
  font-size: 12px;
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

.stamp {
  font-size: 11px;
}
</style>
