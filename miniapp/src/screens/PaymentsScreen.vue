<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { occState, occurrences } from '../stores/occurrences'
import { session, isAdmin } from '../stores/session'
import { groupByWeek } from '../lib/aggregate'
import { weekStart, today } from '../lib/dates'
import Chips from '../components/Chips.vue'
import Section from '../components/Section.vue'
import OccurrenceRow from '../components/OccurrenceRow.vue'
import CurrencyTotals from '../components/CurrencyTotals.vue'
import Skeleton from '../components/Skeleton.vue'
import ErrorState from '../components/ErrorState.vue'
import EmptyState from '../components/EmptyState.vue'
import type { Currency, OccStatus, PaymentResponse } from '../api/types'

const route = useRoute()
const router = useRouter()

type StatusFilter = 'all' | OccStatus
type CurrencyFilter = 'all' | Currency
type MineFilter = 'mine' | 'all'

const statusFilter = computed<StatusFilter>(() => (route.query.status as StatusFilter) || 'all')
const currencyFilter = computed<CurrencyFilter>(() => (route.query.currency as CurrencyFilter) || 'all')
const mineFilter = computed<MineFilter>(() => {
  if (route.query.mine === '1') return 'mine'
  if (route.query.mine === '0') return 'all'
  return isAdmin.value ? 'all' : 'mine'
})
const showPast = computed(() => route.query.past === '1')
// Фильтр по конкретному регулярному платежу (переход из ответа ассистента).
const recFilter = computed(() => (route.query.rec as string | undefined) || null)

function patchQuery(patch: Record<string, string | undefined>): void {
  const q = { ...route.query, ...patch }
  for (const k of Object.keys(q)) if (q[k] === undefined) delete q[k]
  router.replace({ query: q })
}

const todayStr = computed(() => today())
const curWeekStart = computed(() => weekStart(todayStr.value))

const afterScope = computed<PaymentResponse[]>(() => {
  let items = occState.items
  if (recFilter.value) {
    items = items.filter((i) => i.recurrent_payment_id === recFilter.value)
  }
  if (mineFilter.value === 'mine' && session.user) {
    items = items.filter((i) => i.user_id === session.user!.id)
  }
  if (statusFilter.value !== 'all') items = items.filter((i) => i.status === statusFilter.value)
  if (currencyFilter.value !== 'all') items = items.filter((i) => i.currency === currencyFilter.value)
  return items
})

// Ранние: не overdue и due_date < начало текущей недели.
const earlyItems = computed(() =>
  afterScope.value.filter((i) => i.status !== 'overdue' && i.due_date < curWeekStart.value),
)

const visibleItems = computed(() => {
  // При фильтре по регулярному платежу показываем все его периоды, включая прошлые.
  if (showPast.value || recFilter.value) return afterScope.value
  return afterScope.value.filter(
    (i) => i.status === 'overdue' || i.due_date >= curWeekStart.value,
  )
})

const groups = computed(() => groupByWeek(visibleItems.value, todayStr.value))
const firstLoad = computed(() => occState.loading && !occState.loaded)
const hasAnyFilter = computed(
  () =>
    statusFilter.value !== 'all' ||
    currencyFilter.value !== 'all' ||
    route.query.mine !== undefined ||
    !!recFilter.value,
)

function openOcc(id: string): void {
  router.push(`/occ/${id}`)
}
function resetFilters(): void {
  router.replace({ query: {} })
}
function retry(): void {
  void occurrences.load({ force: true })
}

onMounted(() => void occurrences.load())
</script>

<template>
  <div class="payments">
    <h1 class="title">Платежи</h1>

    <div class="filters">
      <Chips
        :model-value="statusFilter"
        :options="[
          { value: 'all', label: 'Все' },
          { value: 'overdue', label: 'Просрочено' },
          { value: 'pending', label: 'Ожидает' },
          { value: 'paid', label: 'Оплачено' },
          { value: 'ended', label: 'Завершён' },
        ]"
        @update:model-value="(v) => patchQuery({ status: v === 'all' ? undefined : v })"
      />
      <Chips
        :model-value="currencyFilter"
        :options="[
          { value: 'all', label: 'Все' },
          { value: 'RUB', label: 'RUB' },
          { value: 'USD', label: 'USD' },
          { value: 'EUR', label: 'EUR' },
        ]"
        @update:model-value="(v) => patchQuery({ currency: v === 'all' ? undefined : v })"
      />
      <Chips
        :model-value="mineFilter"
        :options="[
          { value: 'mine', label: 'Мои' },
          { value: 'all', label: 'Все' },
        ]"
        @update:model-value="(v) => patchQuery({ mine: v === 'mine' ? '1' : '0' })"
      />
    </div>

    <Skeleton v-if="firstLoad" :rows="6" />
    <ErrorState v-else-if="occState.error && !occState.loaded" :error="occState.error" @retry="retry" />
    <template v-else>
      <Section v-for="g in groups" :key="g.weekStart">
        <template #default>
          <div class="wk-head">
            <span>{{ g.label }}</span>
            <CurrencyTotals :totals="g.totals" inline class="wk-sum" />
          </div>
          <OccurrenceRow
            v-for="occ in g.items"
            :key="occ.id"
            :occ="occ"
            show-assignee
            show-status
            @open="openOcc"
          />
        </template>
      </Section>

      <EmptyState
        v-if="!visibleItems.length && (showPast || !earlyItems.length)"
        text="Нет платежей по выбранным фильтрам"
        :action-label="hasAnyFilter ? 'Сбросить фильтры' : undefined"
        @action="resetFilters"
      />
      <p v-else-if="!visibleItems.length" class="empty-note hint">
        Подходящие платежи — в прошлых неделях
      </p>

      <button
        v-if="!showPast && earlyItems.length"
        class="btn btn-secondary earlier"
        type="button"
        @click="patchQuery({ past: '1' })"
      >
        Показать более ранние ({{ earlyItems.length }})
      </button>
    </template>
  </div>
</template>

<style scoped>
.title {
  font-size: 22px;
  margin: 16px 0 12px;
}
.filters {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}
.wk-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  font-weight: 600;
  background: var(--c-secondary-bg);
}
.wk-sum {
  font-size: 14px;
  font-weight: 500;
}
.earlier {
  width: 100%;
  margin: 8px 0 24px;
}
.empty-note {
  padding: 16px 14px;
  margin: 0;
}
</style>
