<script setup lang="ts">
import { computed } from 'vue'
import { monthGrid, monthKey, today } from '../lib/dates'
import { haptic } from '../telegram/webapp'
import type { OccStatus, PaymentResponse } from '../api/types'

const props = defineProps<{
  month: string // YYYY-MM
  selected?: string
  items: PaymentResponse[]
}>()
const emit = defineEmits<{ select: [string] }>()

const WEEKDAYS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
const STATUS_ORDER: OccStatus[] = ['overdue', 'pending', 'paid', 'ended']

const cells = computed(() => monthGrid(props.month))
const todayStr = computed(() => today())

const byDate = computed(() => {
  const map = new Map<string, PaymentResponse[]>()
  for (const it of props.items) {
    const arr = map.get(it.due_date)
    if (arr) arr.push(it)
    else map.set(it.due_date, [it])
  }
  return map
})

function dotsFor(date: string): OccStatus[] {
  const list = byDate.value.get(date)
  if (!list) return []
  const present = new Set(list.map((i) => i.status))
  return STATUS_ORDER.filter((s) => present.has(s)).slice(0, 3)
}

function inMonth(date: string): boolean {
  return monthKey(date) === props.month
}

function pick(date: string): void {
  haptic.selection()
  emit('select', date)
}
</script>

<template>
  <div class="grid-wrap">
    <div class="weekdays">
      <div v-for="w in WEEKDAYS" :key="w" class="wd">{{ w }}</div>
    </div>
    <div class="grid">
      <button
        v-for="d in cells"
        :key="d"
        type="button"
        class="cell"
        :class="{ dim: !inMonth(d), today: d === todayStr, selected: d === selected }"
        @click="pick(d)"
      >
        <span class="daynum">{{ Number(d.slice(8, 10)) }}</span>
        <span class="dots">
          <span v-for="s in dotsFor(d)" :key="s" class="mk" :class="'mk--' + s"></span>
        </span>
      </button>
    </div>
    <div class="legend">
      <span class="lg"><span class="mk mk--overdue"></span>Просрочен</span>
      <span class="lg"><span class="mk mk--pending"></span>Ожидает</span>
      <span class="lg"><span class="mk mk--paid"></span>Оплачен</span>
      <span class="lg"><span class="mk mk--ended"></span>Завершён</span>
    </div>
  </div>
</template>

<style scoped>
.weekdays,
.grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}
.wd {
  text-align: center;
  color: var(--fog);
  font-size: var(--fs-micro);
  padding: 4px 0;
}
.cell {
  aspect-ratio: 1;
  border: 0;
  background: transparent;
  color: var(--bone);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  border-radius: 14px;
  position: relative;
  transition: background-color var(--dur-focus) var(--ease-press);
}
.cell.dim {
  color: var(--fog);
  opacity: 1;
}
.cell.today {
  box-shadow: inset 0 0 0 1px var(--line-strong);
  font-weight: 500;
}
.cell.selected {
  background: var(--accent-bg);
  box-shadow: inset 0 0 0 1px var(--accent);
}
.cell:active {
  background: var(--press);
}
.daynum {
  font-size: var(--fs-body-l);
  font-variant-numeric: tabular-nums;
}
.dots {
  display: flex;
  gap: 3px;
  height: 6px;
  align-items: center;
}
.mk {
  width: 5px;
  height: 5px;
}
.mk--overdue {
  background: var(--red);
  border-radius: 1px;
}
.mk--pending {
  border: 1px solid var(--accent);
  border-radius: 50%;
}
.mk--paid {
  background: var(--green);
  border-radius: 50%;
}
.mk--ended {
  width: 6px;
  height: 1px;
  background: var(--muted);
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: var(--sp-4);
  font-size: var(--fs-micro);
  color: var(--fog);
}
.lg {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
