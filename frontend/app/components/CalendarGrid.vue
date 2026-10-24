<script setup lang="ts">
import type { Period } from '~/utils/periods'

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

// the payment after the selected one on that day, back to the first after the last.
// a day with nothing selected yet starts from its first payment
function pickNext(onDay: Period[]) {
  if (!onDay.length) return
  const at = onDay.findIndex(period => period.id === selected.value?.id)
  select(onDay[(at + 1) % onDay.length]!)
}

const weekdays = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

// as many weeks as the month touches (5 or 6), starting from the monday before the 1st
const weeks = computed(() => {
  const shift = (new Date(props.year, props.month, 1).getDay() + 6) % 7
  const days = new Date(props.year, props.month + 1, 0).getDate()
  return Math.ceil((shift + days) / 7)
})

// the payments of a day share the free part of the cell (rows are 124px, about 80px left
// under the day number): one payment fills it, more split it into stripes. when stripes
// would get thinner than 16px they go into a grid of 2, 3, then 4 columns
const area = 80
const gap = 3

function layout(count: number) {
  let cols = 1
  while (cols < 4 && (area - (Math.ceil(count / cols) - 1) * gap) / Math.ceil(count / cols) < 16) cols++
  const rows = Math.ceil(count / cols)
  const height = (area - (rows - 1) * gap) / rows
  // a tall stripe lets a long name wrap, 15px per line. with two lines or more
  // the last one is the amount under the name
  const lines = Math.max(1, Math.floor((height - 6) / 15))
  const amount = cols === 1 && lines > 1
  return { cols, rows, lines: amount ? lines - 1 : lines, amount, small: height < 22 || cols > 1 }
}

// "120 000,00 ₽" -> "120 000 ₽", a cell is narrow
const shortAmount = (amount: string) => amount.replace(/,00(?=\s)/, '')

const cells = computed(() => monthCells(props.year, props.month, weeks.value).map((cell) => {
  const onDay = cell.inMonth ? props.periods.filter(p => fromRu(p.due) === cell.iso) : []
  return {
    ...cell,
    key: cell.iso,
    day: String(cell.day).padStart(2, '0'),
    today: cell.inMonth && cell.day === props.today,
    payments: onDay,
    layout: layout(onDay.length),
  }
}))
</script>

<template>
  <div class="grid" :style="{ '--weeks': weeks }">
    <span v-for="name in weekdays" :key="name" class="weekday" :class="{ off: name === 'Сб' || name === 'Вс' }">
      {{ name }}
    </span>

    <!-- a click on the day itself, not on a payment, goes through that day's payments one by one -->
    <div
      v-for="(cell, index) in cells"
      :key="cell.key"
      class="cell"
      :class="{ weekend: cell.weekend, pickable: cell.payments.length }"
      @click="pickNext(cell.payments)"
    >
      <span v-if="cell.today" class="today"><span class="circle">{{ cell.day }}</span>сегодня</span>
      <span v-else class="day mono" :class="{ other: !cell.inMonth }">{{ cell.day }}</span>

      <!-- only the name here, amount and the rest are in the side panel -->
      <div
        v-if="cell.payments.length"
        class="payments"
        :class="{ small: cell.layout.small, grid2: cell.layout.cols === 2, tiles: cell.layout.cols > 2 }"
        :style="{ '--cols': cell.layout.cols, '--rows': cell.layout.rows, '--lines': cell.layout.lines, '--d': index % 7 + Math.floor(index / 7) }"
      >
        <button
          v-for="period in cell.payments"
          :key="period.id"
          type="button"
          class="payment"
          :class="[period.status, { selected: selected?.id === period.id }]"
          :title="`${period.title} · ${period.amount}`"
          @click.stop="select(period)"
        >
          <span class="dot" />
          <span class="text">
            <span class="name">{{ period.title }}</span>
            <span v-if="cell.layout.amount" class="amount mono">{{ shortAmount(period.amount) }}</span>
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  /* the days are clicked, not copied */
  -webkit-user-select: none;
  user-select: none;
  grid-template-columns: repeat(7, 1fr);
  grid-template-rows: 30px repeat(var(--weeks), 124px);
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
}

.weekday {
  padding: 7px 12px;
  border-bottom: 1px solid var(--line);
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
}

.weekday.off {
  color: var(--ink-3);
  opacity: 0.7;
}

.cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 10px 6px 6px;
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

.cell.pickable {
  cursor: pointer;
}

.cell.weekend {
  background: color-mix(in srgb, var(--subtle) 50%, transparent);
}

.day {
  margin-bottom: 4px;
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
  font-size: 11px;
  font-weight: 500;
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

.payments {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
  grid-template-rows: repeat(var(--rows), minmax(0, 1fr));
  gap: 3px;
}

/* fills its share of the cell. the corners stay round, a thin stripe ends up a pill */
.payment {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  min-height: 0;
  padding: 0 7px;
  border: 1px solid transparent;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 500;
  line-height: 15px;
  text-align: left;
  transition: border-color 0.2s ease, filter 0.2s ease, transform 0.35s var(--ease-spring), box-shadow 0.35s var(--ease-out);
}

/* lifts off the paper a little, with a soft shadow in its own colour */
.payment:hover {
  filter: brightness(1.08);
  transform: translateY(-1px);
  box-shadow: 0 6px 14px -8px currentColor;
}

.payment:active {
  transform: translateY(0) scale(0.98);
  box-shadow: none;
  transition-duration: 0.1s;
}

/* the name wraps if the stripe is tall enough, the last line ends with … */
.text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.amount {
  overflow: hidden;
  font-size: 11px;
  font-weight: 400;
  opacity: 0.8;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name {
  display: -webkit-box;
  min-width: 0;
  overflow: hidden;
  /* break a long word with a hyphen, not just anywhere */
  hyphens: auto;
  overflow-wrap: break-word;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: var(--lines);
}

.dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

/* thin stripes or a grid: smaller text so the names still read */
.small .payment {
  gap: 4px;
  padding: 0 6px;
  border-radius: 8px;
  font-size: 11px;
}

/* in a grid the colour already tells the status, the dot only takes room */
.grid2 .dot,
.tiles .dot {
  display: none;
}

/* 3 and 4 columns are too narrow for a name: just the coloured tiles, the name is in the tooltip */
.tiles .payment {
  padding: 0;
  border-radius: 6px;
}

.tiles .name {
  display: none;
}

/* same colours as the status badges */
.payment.pending { background: var(--accent-bg); color: var(--accent-ink); }
.payment.paid { background: var(--green-bg); color: var(--green); }
.payment.late { background: var(--amber-bg); color: var(--amber); }
.payment.overdue { background: var(--red-bg); color: var(--red); }
.payment.ended { background: var(--subtle); color: var(--ink-3); }

/* picked: the outline appears and the plaque gives a small bounce */
.payment.selected {
  border-color: currentColor;
  animation: picked 0.45s var(--ease-spring);
}

@keyframes picked {
  40% {
    transform: scale(1.06);
  }
}

/* a new month: the plaques pop in diagonally, from the top left corner to the bottom right */
.payments {
  animation: plaques-in 0.5s var(--ease-spring) backwards;
  animation-delay: calc(var(--d) * 30ms);
}

@keyframes plaques-in {
  from {
    opacity: 0;
    transform: translateY(6px) scale(0.9);
  }
}
</style>
