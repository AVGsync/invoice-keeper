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
const STATUS_VAR: Record<OccStatus, string> = {
  overdue: '--c-overdue',
  pending: '--c-pending',
  paid: '--c-paid',
  ended: '--c-ended',
}

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

function dotsFor(date: string): string[] {
  const list = byDate.value.get(date)
  if (!list) return []
  const present = new Set(list.map((i) => i.status))
  return STATUS_ORDER.filter((s) => present.has(s))
    .slice(0, 3)
    .map((s) => STATUS_VAR[s])
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
          <span
            v-for="(v, i) in dotsFor(d)"
            :key="i"
            class="dot"
            :style="{ background: `var(${v})` }"
          ></span>
        </span>
      </button>
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
  color: var(--c-hint);
  font-size: 12px;
  padding: 4px 0;
}
.cell {
  aspect-ratio: 1;
  border: 0;
  background: transparent;
  color: var(--c-text);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  border-radius: 10px;
  position: relative;
}
.cell.dim {
  color: var(--c-hint);
  opacity: 0.45;
}
.cell.today .daynum {
  outline: 2px solid var(--c-primary);
  border-radius: 50%;
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.cell.selected {
  background: var(--c-secondary-bg);
}
.daynum {
  font-size: 15px;
}
.dots {
  display: flex;
  gap: 2px;
  height: 6px;
}
.dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}
</style>
