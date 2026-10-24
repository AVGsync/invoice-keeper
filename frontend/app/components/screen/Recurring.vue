<script setup lang="ts">
import type { RecurringSearchParams } from '~/types/api'
import type { Template } from '~/utils/periods'
import { noTemplateColumnFilters } from '~/utils/periods'

const { isAdmin } = useRole()
const showNewPayment = ref(false)

const payments = usePayments()
onMounted(payments.load)

// the same filters as on «Периоды»: status, currencies, who pays.
// stopped payments stay in history, the list opens on the running ones
const status = ref('active')
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

// the search pills (name, amount, frequency, next due date). with any of them set the server does the search,
// sending the other filters along (currencies, who pays); the status is still picked here
const columns = ref(noTemplateColumnFilters())
const columnsOn = computed(() => Object.values(columns.value).some(value => value.trim()))
const symbols: Record<string, string> = { RUB: '₽', USD: '$', EUR: '€' }
// the server compares amounts only within one currency
const oneCurrency = computed(() => (currencies.value.length === 1 ? currencies.value[0]! : ''))
const minor = (text: string) => (text ? Math.round(parseMoney(text) * 100) : undefined)

// the server knows two kinds: by weeks and by months (quarterly and yearly are «every 3 / 12 months»)
const frequencyOptions = [
  { value: 'weekly', label: 'Еженедельно' },
  { value: 'monthly', label: 'Ежемесячно, ежеквартально, ежегодно' },
]

const search = useServerSearch(() => {
  if (!columnsOn.value) return null
  const c = columns.value
  const id = payments.assignees.value.find(item => item.person.name === assignee.value)?.id
  const params: RecurringSearchParams = {
    q: c.title.trim() || undefined,
    currency: currencies.value,
    assignee_id: id ? [id] : undefined,
    frequency: c.frequency ? [c.frequency] : undefined,
    amount_min: oneCurrency.value ? minor(c.amountFrom) : undefined,
    amount_max: oneCurrency.value ? minor(c.amountTo) : undefined,
    next_due_from: c.nextFrom || undefined,
    next_due_to: c.nextTo || undefined,
    include_stopped: true,
    limit: 200,
  }
  return params
}, payments.searchTemplates)

// currency and assignee first, the status filter goes on top of that
const base = computed<Template[]>(() => (columnsOn.value
  ? search.result.value?.templates ?? []
  : payments.templates.value))
const filtered = computed(() => base.value.filter(item => currencies.value.includes(item.currency)
  && (!assignee.value || item.who.name === assignee.value)))

// no counts until the payments arrive, a row of zeros would look like there is nothing
const statusOptions = computed(() => {
  const loaded = payments.ready.value
  const stopped = filtered.value.filter(item => item.ended).length
  return [
    { value: 'all', label: 'Все', count: loaded ? filtered.value.length : undefined },
    { value: 'active', label: 'Активные', count: loaded ? filtered.value.length - stopped : undefined },
    { value: 'ended', label: 'Прекращённые', count: loaded ? stopped : undefined },
  ]
})

const shown = computed(() => filtered.value.filter(item => status.value === 'all'
  || (status.value === 'ended' ? item.ended : !item.ended)))

const { withTransition, transitioned } = useViewTransition()
const statusModel = transitioned(status)
const currenciesModel = transitioned(currencies)
const assigneeModel = transitioned(assignee)

const isFiltered = computed(() => status.value !== 'active' || currencies.value.length < 3 || !!assignee.value || columnsOn.value)
function resetFilters() {
  withTransition(() => {
    status.value = 'active'
    currencies.value = ['RUB', 'USD', 'EUR']
    assignee.value = ''
    columns.value = noTemplateColumnFilters()
  })
}

// grey rows: the first load, or the first answer of a search
const loadingNow = computed(() => payments.pending.value || (columnsOn.value && search.searching.value && !search.result.value))

// the server found more than one page: say so instead of quietly showing part of it
const more = computed(() => (columnsOn.value && search.result.value && search.result.value.total > search.result.value.templates.length
  ? search.result.value.total - search.result.value.templates.length
  : 0))

function stop(template: Template) {
  payments.stop(template).catch(() => {})
}
</script>

<template>
  <div>
    <AppPageHeader
      title="Регулярные платежи"
      :subtitle="isAdmin
        ? 'Шаблоны: из них автоматически создаются периоды и напоминания'
        : 'Что, сколько и кто платит — периоды создаются автоматически'"
    >
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <div class="filters">
      <UiSegmented v-model="statusModel" :options="statusOptions" />
      <UiChips v-model="currenciesModel" :options="currencyOptions" />
      <UiDropdown v-model="assigneeModel" :options="assigneeOptions" prefix="Ответственный:" placeholder="все" compact class="assignee" />
    </div>
    <div class="filters">
      <FilterPill v-model="columns.title" label="Название" />
      <FilterPill
        v-model:from="columns.amountFrom"
        v-model:to="columns.amountTo"
        label="Сумма"
        range="amount"
        :unit="oneCurrency ? symbols[oneCurrency] : ''"
        :blocked="oneCurrency ? '' : 'Сумму можно искать в одной валюте: оставьте одну в валютах выше'"
      />
      <FilterPill v-model="columns.frequency" label="Периодичность" :options="frequencyOptions" />
      <FilterPill v-model:from="columns.nextFrom" v-model:to="columns.nextTo" label="Следующий срок" range="date" />
    </div>

    <UiState v-if="payments.loadError.value" kind="error" class="state alone" @retry="payments.load">
      Не удалось загрузить: {{ payments.loadError.value }}
    </UiState>
    <!-- the table stays even when nothing matched: its column titles are where the filters are -->
    <template v-else>
      <RecurringTable
        :templates="loadingNow ? [] : shown"
        :loading="loadingNow"
        @stop="stop"
      />
      <UiState v-if="columnsOn && search.error.value" kind="error" class="state" @retry="search.retry">
        Не удалось найти платежи: {{ search.error.value }}
      </UiState>
      <UiState v-else-if="!loadingNow && !shown.length && isFiltered" class="state">
        Под эти фильтры нет ни одного платежа.
        <br>
        <button type="button" class="reset" @click="resetFilters">Сбросить фильтры</button>
      </UiState>
      <UiState v-else-if="!loadingNow && !shown.length" class="state">
        {{ isAdmin ? 'Регулярных платежей пока нет. Добавьте первый — «Новый платёж».' : 'Регулярных платежей пока нет.' }}
      </UiState>
      <p v-if="more" class="more">Ещё {{ more }} не показаны: уточните фильтры</p>
    </template>

    <p v-if="!isAdmin" class="read-only">
      <span class="dot" />
      Только просмотр: добавлять и прекращать регулярные платежи может администратор.
    </p>

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

.more {
  margin-top: 12px;
  font-size: 13px;
  color: var(--ink-3);
  text-align: center;
}

.reset {
  margin-top: 10px;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
}

.read-only {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 22px;
  font-size: 13px;
  color: var(--ink-3);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ink-3);
}

.new {
  width: 160px;
}

/* under the table, its bottom hairline is already the line above */
.state {
  border-bottom: 1px solid var(--line);
}

.state.alone {
  border-top: 1px solid var(--line);
}
</style>
