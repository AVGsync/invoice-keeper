<script setup lang="ts">
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

// currency and assignee filters first, the status filter goes on top of that
const filtered = computed(() => payments.periods.value.filter(period => fromRu(period.due).startsWith(month.value)
  && currencies.value.includes(period.currency)
  && (!assignee.value || period.who.name === assignee.value)))

const count = (s: string) => filtered.value.filter(period => s === 'paid'
  ? period.status === 'paid' || period.status === 'late'
  : period.status === s).length

const statusOptions = computed(() => [
  { value: 'all', label: 'Все', count: filtered.value.length },
  { value: 'pending', label: 'Не оплачено', count: count('pending') + count('overdue') },
  { value: 'paid', label: 'Оплачено', count: count('paid') },
  { value: 'overdue', label: 'Просрочено', count: count('overdue'), danger: count('overdue') > 0 },
])

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

// any filter narrower than "everything"
const isFiltered = computed(() => month.value !== thisMonth.value || status.value !== 'all' || currencies.value.length < 3 || !!assignee.value)

function resetFilters() {
  withTransition(() => {
    month.value = thisMonth.value
    status.value = 'all'
    currencies.value = ['RUB', 'USD', 'EUR']
    assignee.value = ''
  })
}

// footer totals for the whole month, counted from the filtered rows
const monthName = computed(() => monthNames[parseMonthKey(month.value).month]!.toLowerCase())
const shownTotals = computed(() => totalsOn(lastDayOf(month.value), filtered.value)
  .filter(total => filtered.value.some(period => period.currency === total.code)))
</script>

<template>
  <div>
    <AppPageHeader title="Платежи" subtitle="Периоды регулярных платежей: срок, оплата и просрочка">
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <div class="filters">
      <UiMonthPicker v-model="monthModel" :current="thisMonth" />
      <UiSegmented v-model="statusModel" :options="statusOptions" />
      <UiChips v-model="currenciesModel" :options="currencyOptions" />
      <UiDropdown v-model="assigneeModel" :options="assigneeOptions" prefix="Ответственный:" placeholder="все" compact class="assignee" />
    </div>

    <UiState v-if="payments.loadError.value" kind="error" class="load-error" @retry="payments.load">
      Не удалось загрузить платежи: {{ payments.loadError.value }}
    </UiState>
    <PaymentsTable
      :periods="shown"
      :totals="shownTotals"
      :filtered="isFiltered"
      :month="monthName"
      :loading="payments.pending.value"
      @reset="resetFilters"
    />

    <Transition name="modal">
      <NewPaymentModal v-if="showNewPayment" @close="showNewPayment = false" />
    </Transition>
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
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

.load-error {
  margin-bottom: 12px;
}
</style>
