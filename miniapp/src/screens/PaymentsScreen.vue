<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
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

// Поиск по названию и ответственному; хранится в адресе (?q=), чтобы переживать переходы.
const query = computed(() => ((route.query.q as string | undefined) ?? '').trim().toLowerCase())
const search = ref((route.query.q as string | undefined) ?? '')
watch(search, (v) => patchQuery({ q: v.trim() || undefined }))
watch(
  () => route.query.q,
  (v) => {
    if (((v as string | undefined) ?? '') !== search.value.trim()) search.value = (v as string | undefined) ?? ''
  },
)

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
  if (query.value) {
    items = items.filter(
      (i) =>
        i.description.toLowerCase().includes(query.value) ||
        (i.assignee_name ?? '').toLowerCase().includes(query.value),
    )
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
  if (showPast.value || recFilter.value || query.value) return afterScope.value
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
    !!query.value ||
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

    <input
      v-model="search"
      class="search"
      type="search"
      enterkeyhint="search"
      autocomplete="off"
      placeholder="Поиск по названию или ответственному"
      aria-label="Поиск платежей"
    />

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
.search {
  margin-bottom: var(--sp-3);
  border-radius: var(--radius-pill);
  padding-left: var(--sp-4);
}
.filters {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
  margin-bottom: var(--sp-2);
}
.payments :deep(.section):first-of-type {
  margin-top: var(--sp-5);
}
.payments :deep(.section + .section) {
  margin-top: var(--sp-3);
}
.wk-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--sp-3);
  padding: var(--sp-4) 0 var(--sp-3);
  border-bottom: 1px solid var(--c-line);
}
.wk-head > span:first-child {
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--muted);
}
.wk-sum {
  font-size: var(--fs-caption);
  color: var(--ash-text);
}
.earlier {
  width: 100%;
  margin: var(--sp-6) 0;
}
.empty-note {
  padding: var(--sp-4) 0;
  margin: 0;
}
</style>
