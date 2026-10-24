<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { occState, occurrences } from '../stores/occurrences'
import { session } from '../stores/session'
import { monthKey, today, formatMonth, addMonths } from '../lib/dates'
import { sumByCurrency } from '../lib/money'
import Segmented from '../components/Segmented.vue'
import MonthGrid from '../components/MonthGrid.vue'
import Section from '../components/Section.vue'
import OccurrenceRow from '../components/OccurrenceRow.vue'
import CurrencyTotals from '../components/CurrencyTotals.vue'
import Skeleton from '../components/Skeleton.vue'
import ErrorState from '../components/ErrorState.vue'
import type { PaymentResponse } from '../api/types'

const route = useRoute()
const router = useRouter()

const month = computed(() => (route.query.m as string) || monthKey(today()))
const selected = computed(() => route.query.d as string | undefined)
type Scope = 'mine' | 'all'
const scope = computed<Scope>(() => (route.query.mine === '0' ? 'all' : 'mine'))

function patch(p: Record<string, string | undefined>): void {
  const q = { ...route.query, ...p }
  for (const k of Object.keys(q)) if (q[k] === undefined) delete q[k]
  router.replace({ query: q })
}

const base = computed<PaymentResponse[]>(() => {
  if (scope.value === 'mine' && session.user) {
    return occState.items.filter((i) => i.user_id === session.user!.id)
  }
  return occState.items
})

const dayItems = computed(() =>
  selected.value ? base.value.filter((i) => i.due_date === selected.value) : [],
)

// Для итога месяца исключаем ended (как в monthTotals на Today и /payments/summary).
const monthItems = computed(() =>
  base.value.filter((i) => monthKey(i.due_date) === month.value && i.status !== 'ended'),
)
const monthTotalsSum = computed(() =>
  sumByCurrency(monthItems.value, (i) => ({ currency: i.currency, amount: i.amount })),
)

const firstLoad = computed(() => occState.loading && !occState.loaded)

function prevMonth(): void {
  patch({ m: addMonths(month.value, -1), d: undefined })
}
function nextMonth(): void {
  patch({ m: addMonths(month.value, 1), d: undefined })
}
function select(d: string): void {
  patch({ d })
}
function openOcc(id: string): void {
  router.push(`/occ/${id}`)
}
function retry(): void {
  void occurrences.load({ force: true })
}

onMounted(() => void occurrences.load())
</script>

<template>
  <div class="calendar">
    <div class="month-head">
      <button class="nav" type="button" @click="prevMonth">‹</button>
      <h1 class="mtitle">{{ formatMonth(month) }}</h1>
      <button class="nav" type="button" @click="nextMonth">›</button>
    </div>

    <Segmented
      :model-value="scope"
      :options="[
        { value: 'mine', label: 'Мои' },
        { value: 'all', label: 'Все' },
      ]"
      @update:model-value="(v) => patch({ mine: v === 'mine' ? '1' : '0' })"
    />

    <Skeleton v-if="firstLoad" :rows="6" />
    <ErrorState v-else-if="occState.error && !occState.loaded" :error="occState.error" @retry="retry" />
    <template v-else>
      <MonthGrid :month="month" :selected="selected" :items="base" @select="select" />

      <Section v-if="selected">
        <template v-if="dayItems.length">
          <OccurrenceRow
            v-for="occ in dayItems"
            :key="occ.id"
            :occ="occ"
            :show-assignee="scope === 'all'"
            show-status
            @open="openOcc"
          />
        </template>
        <p v-else class="empty-inline">В этот день платежей нет</p>
      </Section>

      <Section v-else :title="'Итого за ' + formatMonth(month).toLowerCase()">
        <div class="totals-pad">
          <CurrencyTotals :totals="monthTotalsSum" />
        </div>
      </Section>
    </template>
  </div>
</template>

<style scoped>
.month-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--sp-3);
  margin: var(--sp-6) 0 var(--sp-5);
}
.mtitle {
  font-size: clamp(28px, 8.5vw, 34px);
  line-height: 1.05;
  letter-spacing: -0.035em;
  flex: 1;
  margin: 0;
  order: 1;
}
.nav {
  width: 44px;
  height: 44px;
  border: 1px solid var(--line-strong);
  border-radius: 50%;
  background: var(--graphite);
  color: var(--bone-text);
  font-size: 18px;
  line-height: 1;
}
.nav:active {
  background: var(--press);
}
.nav:first-of-type {
  order: 2;
}
.nav:last-of-type {
  order: 3;
}
.empty-inline {
  padding: var(--sp-4) 0;
  color: var(--fog);
  margin: 0;
}
.totals-pad {
  padding: var(--sp-4) 0;
  font-size: var(--fs-title);
  font-variant-numeric: tabular-nums;
}
</style>
