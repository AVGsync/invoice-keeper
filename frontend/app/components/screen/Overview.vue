<script setup lang="ts">
import type { Period } from '~/utils/periods'

const { isAdmin } = useRole()

const payments = usePayments()
onMounted(payments.load)

const showAll = ref('all')
// totals are shown as of this date
const asOf = ref(payments.today.value)
const { transitioned } = useViewTransition()
// picking a date crossfades the numbers in the cards
const asOfModel = transitioned(asOf)
const asOfTotals = computed(() => totalsOn(asOf.value, payments.periods.value))
const showNewPayment = ref(false)

// "Суббота, 24 октября 2026"
const todayText = computed(() => {
  const text = new Date(`${payments.today.value}T12:00:00`)
    .toLocaleDateString('ru-RU', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
    .replace(' г.', '')
  return text[0]!.toUpperCase() + text.slice(1)
})

const overdueList = computed(() => payments.periods.value.filter(period => period.status === 'overdue'))
const overdue = computed(() => overdueList.value[0])
const overdueTitle = computed(() => {
  const n = overdueList.value.length
  return `${n === 1 ? 'Просрочен' : 'Просрочено'} ${n} ${plural(n, ['платёж', 'платежа', 'платежей'])}`
})
const owner = computed(() => (overdue.value?.who.name === payments.meName.value ? 'вы' : overdue.value?.who.short))

// due today and in the next 7 days, everyone's or only mine
const upcoming = computed(() => payments.periods.value.filter((period) => {
  const days = daysBetween(payments.today.value, fromRu(period.due))
  const mine = showAll.value === 'all' || period.who.name === payments.meName.value
  return days >= 0 && days <= 7 && period.status !== 'ended' && mine
}))

// a failed mark shows up in the corner note (usePayments), nothing else to do here
function mark(period: Period) {
  payments.markPaid(period, payments.today.value).catch(() => {})
}
</script>

<template>
  <div>
    <AppPageHeader title="Обзор" :subtitle="`${todayText} · Europe/Moscow`">
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <OverdueBanner
      v-if="overdue"
      :title="overdueTitle"
      :text="`${overdue.title} · срок ${overdue.due.slice(0, 5)} · просрочка ${overdue.overdue} · ответственный: ${owner}`"
      :amount="overdue.amount"
      :can-mark="payments.canMark(overdue)"
      @mark="mark(overdue)"
    />

    <AppSectionTitle index="01" title="Итоги по валютам" class="section">
      <template #right>
        <span class="as-of">
          <span class="muted">По дате</span>
          <UiDatePicker v-model="asOfModel" :today="payments.today.value" class="as-of-picker" />
        </span>
      </template>
    </AppSectionTitle>

    <div class="cards">
      <CurrencyCard
        v-for="(total, i) in asOfTotals"
        :key="total.code"
        class="rise"
        :total="total"
        :label="`К оплате по ${toRu(asOf)}¹`"
        :loading="!payments.ready.value"
        :style="{ viewTransitionName: `card-${total.code}`, '--i': i }"
      />
    </div>
    <p class="kicker footnote">¹ срок до выбранной даты и то, что оплачено раньше срока</p>

    <AppSectionTitle index="02" title="Ближайшие 7 дней" class="section">
      <UiSegmented
        v-model="showAll"
        :options="[{ value: 'all', label: 'Все' }, { value: 'mine', label: 'Мои' }]"
        class="toggle"
      />
      <template #right>
        <NuxtLink :to="isAdmin ? '/admin/payments' : '/user/payments'" class="all-link">Все платежи <UiIcon name="arrow" :size="14" class="all-arrow" /></NuxtLink>
      </template>
    </AppSectionTitle>

    <UpcomingList v-if="payments.pending.value" :items="[]" loading />
    <UiState v-else-if="payments.loadError.value" kind="error" class="upcoming-state" @retry="payments.load">
      Не удалось загрузить платежи: {{ payments.loadError.value }}
    </UiState>
    <UiState v-else-if="!upcoming.length" pose="rest" class="upcoming-state">На ближайшую неделю платежей нет.</UiState>
    <UpcomingList v-else :items="upcoming" :can-mark="payments.canMark" @mark="mark" />

    <Transition name="modal">
      <NewPaymentModal v-if="showNewPayment" @close="showNewPayment = false" />
    </Transition>
  </div>
</template>

<style scoped>
.section {
  margin: 30px 0 18px;
}

.cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.as-of {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.as-of-picker {
  width: 150px;
}

.footnote {
  margin-top: 12px;
}

.toggle {
  margin-left: 6px;
}

.all-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ink-2);
  transition: color 0.2s ease;
}

.all-link:hover {
  color: var(--ink);
}

/* the arrow steps forward on hover */
.all-arrow {
  transition: transform 0.35s var(--ease-out);
}

.all-link:hover .all-arrow {
  transform: translateX(3px);
}

.new {
  width: 160px;
}

.upcoming-state {
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
}

/* phones */
@media (max-width: 1240px) {
  .section {
    margin: 24px 0 12px;
  }

  .cards {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .as-of-picker {
    width: 140px;
  }
}

/* a tablet: the three currencies side by side again */
@media (min-width: 700px) and (max-width: 1240px) {
  .cards {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
