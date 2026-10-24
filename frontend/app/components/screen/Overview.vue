<script setup lang="ts">
const { isAdmin } = useRole()

const showAll = ref('all')
// totals are shown as of this date
const asOf = ref(demoToday)
const { transitioned } = useViewTransition()
// picking a date crossfades the numbers in the cards
const asOfModel = transitioned(asOf)
const asOfTotals = computed(() => totalsOn(asOf.value))
const showNewPayment = ref(false)

const overdue = periods.find(period => period.status === 'overdue')!
const owner = computed(() => isAdmin.value ? overdue.who.short : 'вы')
</script>

<template>
  <div>
    <AppPageHeader title="Обзор" subtitle="Суббота, 24 октября 2026 · Europe/Moscow">
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <OverdueBanner
      title="Просрочен 1 платёж"
      :text="`${overdue.title} · срок ${overdue.due.slice(0, 5)} · просрочка ${overdue.overdue} · ответственная — ${owner}`"
      :amount="overdue.amount"
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
        v-for="total in asOfTotals"
        :key="total.code"
        :total="total"
        :label="`К оплате по ${toRu(asOf)}¹`"
        :style="{ viewTransitionName: `card-${total.code}` }"
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
        <NuxtLink :to="isAdmin ? '/admin/payments' : '/user/payments'" class="all-link">Все платежи&nbsp; →</NuxtLink>
      </template>
    </AppSectionTitle>

    <UpcomingList :items="upcoming" />

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
  font-size: 13px;
  color: var(--ink-2);
}

.new {
  width: 160px;
}
</style>
