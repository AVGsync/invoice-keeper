<script setup lang="ts">
const { isAdmin } = useRole()

const showAll = ref('all')
// totals are shown as of this date
const asOf = ref(demoToday)
const { transitioned } = useViewTransition()
// picking a date crossfades the numbers in the cards
const asOfModel = transitioned(asOf)
const demo = useDemo()
const asOfTotals = computed(() => totalsOn(asOf.value, demo.periods.value))
const showNewPayment = ref(false)

const overdueList = computed(() => demo.periods.value.filter(period => period.status === 'overdue'))
const overdue = computed(() => overdueList.value[0])
const overdueTitle = computed(() => {
  const n = overdueList.value.length
  return `${n === 1 ? 'Просрочен' : 'Просрочено'} ${n} ${plural(n, ['платёж', 'платежа', 'платежей'])}`
})
const owner = computed(() => (isAdmin.value ? overdue.value?.who.short : 'вы'))

// due in the next 7 days
const upcoming = computed(() => demo.periods.value.filter((period) => {
  const days = daysBetween(demoToday, fromRu(period.due))
  return days >= 1 && days <= 7
}))
</script>

<template>
  <div>
    <AppPageHeader title="Обзор" subtitle="Суббота, 24 октября 2026 · Europe/Moscow">
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <OverdueBanner
      v-if="overdue"
      :title="overdueTitle"
      :text="`${overdue.title} · срок ${overdue.due.slice(0, 5)} · просрочка ${overdue.overdue} · ответственная — ${owner}`"
      :amount="overdue.amount"
      :soon="!demo.enabled.value"
      @mark="demo.markPaid(overdue, demoToday)"
    />

    <AppSectionTitle index="01" title="Итоги по валютам" hint="валюты не суммируются между собой" class="section">
      <template #right>
        <span class="as-of">
          <span class="muted">на дату</span>
          <UiDatePicker v-model="asOfModel" :today="demoToday" class="as-of-picker" />
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
        :style="{ viewTransitionName: `card-${total.code}`, '--i': i }"
      />
    </div>
    <p class="kicker footnote">¹ срок до выбранной даты и то, что оплачено раньше срока · курсы и конвертация не применяются</p>

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

    <UpcomingList :items="upcoming" :soon="!demo.enabled.value" @mark="period => demo.markPaid(period, demoToday)" />

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
</style>
