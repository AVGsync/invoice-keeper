<script setup lang="ts">
import type { PaymentSearchParams } from '~/types/api'
import type { Period } from '~/utils/periods'
import { noPeriodColumnFilters } from '~/utils/periods'

const { isAdmin } = useRole()

const status = ref('all')
const showNewPayment = ref(false)

const payments = usePayments()
onMounted(payments.load)

// 'YYYY-MM', starts on the current month
const thisMonth = computed(() => payments.today.value.slice(0, 7))
const month = ref(thisMonth.value)

const currencies = ref<string[]>(['RUB', 'USD', 'EUR'])
const currencyOptions = [
  { value: 'RUB', label: 'RUB', symbol: '₽' },
  { value: 'USD', label: 'USD', symbol: '$' },
  { value: 'EUR', label: 'EUR', symbol: '€' },
]

// '' = everyone, otherwise the person's full name
const assignee = ref('')
const assigneeOptions = computed(() => payments.assignees.value
  .map(({ person }) => ({ value: person.name, label: person.short, meta: person.persona })))

// the search pills (name, due date, amount, paid date). with any of them set the server does the search,
// sending the other filters along (month, currencies, who pays)
const columns = ref(noPeriodColumnFilters())
const columnsOn = computed(() => Object.values(columns.value).some(value => value.trim()))
const symbols: Record<string, string> = { RUB: '₽', USD: '$', EUR: '€' }
// the server compares amounts only within one currency
const oneCurrency = computed(() => (currencies.value.length === 1 ? currencies.value[0]! : ''))

const later = (a: string, b: string) => (a > b ? a : b)
const earlier = (a: string, b: string) => (!b || a < b ? a : b)
const minor = (text: string) => (text ? Math.round(parseMoney(text) * 100) : undefined)

const search = useServerSearch(() => {
  if (!columnsOn.value) return null
  const c = columns.value
  const monthStart = `${month.value}-01`
  const id = payments.assignees.value.find(item => item.person.name === assignee.value)?.id
  const params: PaymentSearchParams = {
    q: c.title.trim() || undefined,
    // the month above and the «Срок» range, both at once
    due_from: later(monthStart, c.dueFrom),
    due_to: earlier(lastDayOf(month.value), c.dueTo),
    paid_from: c.paidFrom || undefined,
    paid_to: c.paidTo || undefined,
    currency: currencies.value,
    assignee_id: id ? [id] : undefined,
    amount_min: oneCurrency.value ? minor(c.amountFrom) : undefined,
    amount_max: oneCurrency.value ? minor(c.amountTo) : undefined,
    sort: 'due_date',
    order: 'asc',
    limit: 200,
  }
  return params
}, payments.searchPeriods)

// the rows before the status filter: the server's answer, or the loaded month with the filters above
const base = computed<Period[]>(() => (columnsOn.value
  ? search.result.value?.periods ?? []
  : payments.periods.value))

// currency and assignee filters first, the status filter goes on top of that
const filtered = computed(() => base.value.filter(period => fromRu(period.due).startsWith(month.value)
  && currencies.value.includes(period.currency)
  && (!assignee.value || period.who.name === assignee.value)))

const count = (s: string) => filtered.value.filter(period => s === 'paid'
  ? period.status === 'paid' || period.status === 'late'
  : period.status === s).length

// no counts until the payments arrive, a row of zeros would look like there is nothing
const statusOptions = computed(() => {
  const loaded = payments.ready.value
  return [
    { value: 'all', label: 'Все', count: loaded ? filtered.value.length : undefined },
    { value: 'pending', label: 'Не оплачено', count: loaded ? count('pending') + count('overdue') : undefined },
    { value: 'paid', label: 'Оплачено', count: loaded ? count('paid') : undefined },
    { value: 'overdue', label: 'Просрочено', count: loaded ? count('overdue') : undefined, danger: loaded && count('overdue') > 0 },
  ]
})

const shown = computed(() => {
  if (status.value === 'all') return filtered.value
  if (status.value === 'pending') return filtered.value.filter(period => !period.paidAt)
  if (status.value === 'paid') return filtered.value.filter(period => period.paidAt)
  return filtered.value.filter(period => period.status === 'overdue')
})

const { withTransition, transitioned } = useViewTransition()
const monthModel = transitioned(month)
const statusModel = transitioned(status)
const currenciesModel = transitioned(currencies)
const assigneeModel = transitioned(assignee)

function goToday() {
  monthModel.value = thisMonth.value
}

// any filter narrower than "everything"
const isFiltered = computed(() => month.value !== thisMonth.value || status.value !== 'all' || currencies.value.length < 3 || !!assignee.value || columnsOn.value)

function resetFilters() {
  withTransition(() => {
    month.value = thisMonth.value
    status.value = 'all'
    currencies.value = ['RUB', 'USD', 'EUR']
    assignee.value = ''
    columns.value = noPeriodColumnFilters()
  })
}

// the server found more than one page: say so instead of quietly showing part of it
const more = computed(() => (columnsOn.value && search.result.value && search.result.value.total > search.result.value.periods.length
  ? search.result.value.total - search.result.value.periods.length
  : 0))

// footer totals for the whole month, counted from the filtered rows
const monthName = computed(() => monthNames[parseMonthKey(month.value).month]!.toLowerCase())
const shownTotals = computed(() => totalsOn(lastDayOf(month.value), filtered.value)
  .filter(total => filtered.value.some(period => period.currency === total.code)))
</script>

<template>
  <div>
    <AppPageHeader title="Периоды" subtitle="Сроки регулярных платежей: оплата и просрочка">
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <!-- the month as a big title, like on Календарь -->
    <div class="toolbar">
      <UiMonthPicker v-model="monthModel" variant="title" :current="thisMonth" />
      <!-- back to the current month -->
      <UiButton class="today" aria-label="Сегодня" title="К сегодняшнему месяцу" @click="goToday"><svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
          <!-- drawn around the middle of its box, so it sits centred in the round button -->
          <path d="M8 2.5A5.5 5.5 0 1 1 3.24 5.25M9.9 0.9L8 2.5L9.9 4.1" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        </svg></UiButton>
    </div>

    <div class="filters">
      <UiSegmented v-model="statusModel" :options="statusOptions" />
      <UiChips v-model="currenciesModel" :options="currencyOptions" />
      <UiDropdown v-model="assigneeModel" :options="assigneeOptions" prefix="Ответственный:" placeholder="все" compact class="assignee" />
    </div>
    <div class="filters">
      <FilterPill v-model="columns.title" label="Название" />
      <FilterPill v-model:from="columns.dueFrom" v-model:to="columns.dueTo" label="Срок" range="date" />
      <FilterPill
        v-model:from="columns.amountFrom"
        v-model:to="columns.amountTo"
        label="Сумма"
        range="amount"
        :unit="oneCurrency ? symbols[oneCurrency] : ''"
        :blocked="oneCurrency ? '' : 'Сумму можно искать в одной валюте: оставьте одну в валютах выше'"
      />
      <FilterPill v-model:from="columns.paidFrom" v-model:to="columns.paidTo" label="Дата оплаты" range="date" />
    </div>

    <UiState v-if="payments.loadError.value" kind="error" class="load-error" @retry="payments.load">
      Не удалось загрузить платежи: {{ payments.loadError.value }}
    </UiState>
    <UiState v-else-if="columnsOn && search.error.value" kind="error" class="load-error" @retry="search.retry">
      Не удалось найти платежи: {{ search.error.value }}
    </UiState>
    <!-- after a failed load the table would say «no payments», which isn't true -->
    <PaymentsTable
      v-if="!payments.loadError.value"
      :periods="shown"
      :totals="shownTotals"
      :filtered="isFiltered"
      :month="monthName"
      :loading="payments.pending.value || (columnsOn && search.searching.value && !search.result.value)"
      @reset="resetFilters"
    />
    <p v-if="more" class="more">Ещё {{ more }} не показаны: уточните фильтры</p>

    <Transition name="modal">
      <NewPaymentModal v-if="showNewPayment" @close="showNewPayment = false" />
    </Transition>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
}

.today {
  display: grid;
  place-items: center;
  height: 32px;
  width: 32px;
  padding: 0;
  color: var(--ink-2);
}

.today:hover {
  color: var(--ink);
}

.filters {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}

/* the search pills, a second row under the main filters */
.filters + .filters {
  gap: 10px;
  margin-top: -6px;
}

/* grows to fit the picked name, up to a limit */
.assignee {
  flex: 0 1 auto;
  min-width: 220px;
  max-width: 280px;
  margin-left: auto;
}

.new {
  width: 160px;
}

.more {
  margin-top: 12px;
  font-size: 13px;
  color: var(--ink-3);
  text-align: center;
}

.load-error {
  margin-bottom: 12px;
}

/* phones */
@media (max-width: 1240px) {
  .toolbar {
    margin-bottom: 14px;
  }

  .filters {
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 12px;
  }

  .filters + .filters {
    margin-top: 0;
  }

  /* the status switch can be wider than the screen: it scrolls sideways */
  .filters > :first-child {
    max-width: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .assignee {
    flex: 1 1 100%;
    min-width: 0;
    max-width: none;
    margin-left: 0;
  }

  .new {
    width: auto;
    flex: 1 1 auto;
    height: 44px;
  }
}
</style>
