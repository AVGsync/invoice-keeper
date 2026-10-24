<script setup lang="ts">
import type { Period } from '~/utils/demo'

const { withTransition } = useViewTransition()

const props = defineProps<{
  year: number
  // 0 = january
  month: number
  today: number
  periods: Period[]
}>()

const selected = defineModel<Period>('selected')

// the side panel morphs to the new payment instead of jumping
function select(period: Period) {
  withTransition(() => { selected.value = period })
}

const weekdays = ['ПН', 'ВТ', 'СР', 'ЧТ', 'ПТ', 'СБ', 'ВС']

const short = { pending: 'ЖДЁТ', paid: 'ОПЛ.', late: 'ОПЛ.', overdue: 'ПРОСР.' }

function label(period: Period) {
  const extra = period.status === 'late' ? ` ${period.overdue?.replace(' дн.', '')}` : ''
  return `${period.who.initials} · ${short[period.status]}${extra}`
}

// 5 weeks starting from the monday before the 1st
const cells = computed(() => monthCells(props.year, props.month, 5).map(cell => ({
  ...cell,
  key: cell.iso,
  day: String(cell.day).padStart(2, '0'),
  today: cell.inMonth && cell.day === props.today,
  period: cell.inMonth ? props.periods.find(p => fromRu(p.due) === cell.iso) : undefined,
})))
</script>

<template>
  <div class="card grid">
    <span v-for="name in weekdays" :key="name" class="weekday mono" :class="{ off: name === 'СБ' || name === 'ВС' }">
      {{ name }}
    </span>

    <div v-for="cell in cells" :key="cell.key" class="cell" :class="{ weekend: cell.weekend }">
      <span v-if="cell.today" class="today mono"><span class="circle">{{ cell.day }}</span>СЕГОДНЯ</span>
      <span v-else class="day mono" :class="{ other: !cell.inMonth }">{{ cell.day }}</span>

      <button
        v-if="cell.period"
        type="button"
        class="payment"
        :class="[cell.period.status, { selected: selected === cell.period }]"
        @click="select(cell.period)"
      >
        <span class="title">{{ cell.period.title }}</span>
        <span class="amount mono">{{ cell.period.amount }}</span>
        <span class="who mono">{{ label(cell.period) }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  grid-template-rows: 30px repeat(5, 136px);
  overflow: hidden;
}

.weekday {
  padding: 8px 12px;
  background: var(--subtle);
  border-bottom: 1px solid var(--line);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.08em;
  color: var(--ink-2);
}

.weekday.off {
  color: var(--ink-3);
}

.cell {
  display: flex;
  flex-direction: column;
  gap: 15px;
  padding: 12px 6px 6px;
  border-top: 1px solid var(--line);
  border-left: 1px solid var(--line);
}

.cell:nth-child(7n + 1) {
  border-left: 0;
}

/* first week row has no line above, the weekday header has one */
.cell:nth-child(-n + 14) {
  border-top: 0;
}

.cell.weekend {
  background: color-mix(in srgb, var(--subtle) 60%, transparent);
}

.day {
  padding-left: 6px;
  font-size: 13px;
  font-weight: 500;
}

.day.other {
  color: var(--ink-3);
  font-weight: 400;
}

.today {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: -4px 0 -5px;
  font-size: 9px;
  letter-spacing: 0.08em;
  color: var(--accent-ink);
}

.circle {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: 0;
  line-height: 26px;
  text-align: center;
}

.payment {
  display: flex;
  flex-direction: column;
  height: 78px;
  padding: 8px 8px 8px 11px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-left: 3px solid var(--ink-3);
  border-radius: 6px;
  overflow: hidden;
  text-align: left;
}

.payment.selected {
  border-color: var(--accent-ink);
  border-left-color: var(--ink-3);
}

.payment.paid { border-left-color: var(--green); }
.payment.late { border-left-color: var(--amber); }
.payment.overdue {
  background: var(--red-bg);
  border-left-color: var(--red);
}

/* long names stop after 2 lines so amount and status always fit in the card */
.title {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  font-size: 12px;
  font-weight: 500;
  line-height: 16px;
}

.amount {
  margin-top: auto;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
}

.who {
  margin-top: 2px;
  font-size: 9px;
  letter-spacing: 0.06em;
  color: var(--ink-3);
}

.paid .who { color: var(--green); }
.late .who { color: var(--amber); }
.overdue .who { color: var(--red); }
</style>
