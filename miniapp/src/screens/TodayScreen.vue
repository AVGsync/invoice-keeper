<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { occState, occurrences } from '../stores/occurrences'
import { session } from '../stores/session'
import { upcoming, overdue, monthTotals } from '../lib/aggregate'
import { sumByCurrency } from '../lib/money'
import { today, monthKey, formatMonth } from '../lib/dates'
import Segmented from '../components/Segmented.vue'
import Section from '../components/Section.vue'
import OccurrenceRow from '../components/OccurrenceRow.vue'
import CurrencyTotals from '../components/CurrencyTotals.vue'
import Money from '../components/Money.vue'
import Skeleton from '../components/Skeleton.vue'
import ErrorState from '../components/ErrorState.vue'
import { CURRENCY_ORDER } from '../lib/money'
import type { Currency, PaymentResponse } from '../api/types'

const route = useRoute()
const router = useRouter()

type Scope = 'mine' | 'team'
const scope = computed<Scope>(() => (route.query.mine === '0' ? 'team' : 'mine'))

function setScope(v: Scope): void {
  router.replace({ query: { ...route.query, mine: v === 'mine' ? '1' : '0' } })
}

const base = computed<PaymentResponse[]>(() => {
  if (scope.value === 'mine' && session.user) {
    return occState.items.filter((i) => i.user_id === session.user!.id)
  }
  return occState.items
})

const todayStr = computed(() => today())
const ym = computed(() => monthKey(todayStr.value))

const overdueItems = computed(() => overdue(base.value))
const overdueTotals = computed(() =>
  sumByCurrency(overdueItems.value, (i) => ({ currency: i.currency, amount: i.amount })),
)
const upcomingItems = computed(() => upcoming(base.value, todayStr.value, 7))
const totals = computed(() => monthTotals(base.value, ym.value))
const totalsCurrencies = computed(() =>
  CURRENCY_ORDER.filter((c) => totals.value[c as Currency]),
)

const firstLoad = computed(() => occState.loading && !occState.loaded)

function openOverdue(): void {
  router.push({ path: '/payments', query: { status: 'overdue', mine: scope.value === 'mine' ? '1' : '0' } })
}
function openOcc(id: string): void {
  router.push(`/occ/${id}`)
}
function retry(): void {
  void occurrences.load({ force: true })
}

// Подхватываем свежие данные при открытии вкладки (не чаще раза в 60 с).
onMounted(() => void occurrences.load())
</script>

<template>
  <div class="today">
    <h1 class="title">Сегодня</h1>
    <Segmented
      :model-value="scope"
      :options="[
        { value: 'mine', label: 'Мои' },
        { value: 'team', label: 'Команда' },
      ]"
      @update:model-value="setScope"
    />

    <Skeleton v-if="firstLoad" :rows="5" />
    <ErrorState v-else-if="occState.error && !occState.loaded" :error="occState.error" @retry="retry" />

    <template v-else>
      <button v-if="overdueItems.length" class="overdue-card" type="button" @click="openOverdue">
        <div class="ov-head">Просрочено: {{ overdueItems.length }}</div>
        <CurrencyTotals :totals="overdueTotals" inline class="ov-sum" />
      </button>

      <Section title="Ближайшие 7 дней">
        <template v-if="upcomingItems.length">
          <OccurrenceRow
            v-for="occ in upcomingItems"
            :key="occ.id"
            :occ="occ"
            :show-assignee="scope === 'team'"
            @open="openOcc"
          />
        </template>
        <p v-else class="empty-inline">На ближайшую неделю платежей нет</p>
      </Section>

      <Section :title="formatMonth(ym)">
        <div v-if="totalsCurrencies.length" class="month">
          <div v-for="c in totalsCurrencies" :key="c" class="mrow">
            <div class="cur">{{ c }}</div>
            <div class="mvals">
              <div>Оплачено <Money :amount="totals[c as Currency]!.paid" :currency="c as Currency" /></div>
              <div>Осталось <Money :amount="totals[c as Currency]!.outstanding" :currency="c as Currency" /></div>
              <div v-if="totals[c as Currency]!.overdue" class="over">
                из них просрочено <Money :amount="totals[c as Currency]!.overdue" :currency="c as Currency" />
              </div>
            </div>
          </div>
        </div>
        <p v-else class="empty-inline">В этом месяце платежей нет</p>
      </Section>
    </template>
  </div>
</template>

<style scoped>
.title {
  font-size: 22px;
  margin: 16px 0 12px;
}
.overdue-card {
  display: block;
  width: 100%;
  text-align: left;
  border: 0;
  border-radius: var(--radius);
  background: color-mix(in srgb, var(--c-overdue) 12%, var(--c-bg));
  color: var(--c-overdue);
  padding: 14px;
  margin: 16px 0;
}
.ov-head {
  font-weight: 700;
  margin-bottom: 4px;
}
.ov-sum {
  font-weight: 600;
}
.empty-inline {
  padding: 16px 14px;
  color: var(--c-hint);
  margin: 0;
}
.month {
  padding: 12px 14px;
}
.mrow {
  display: flex;
  gap: 12px;
  padding: 6px 0;
}
.cur {
  font-weight: 600;
  width: 44px;
}
.mvals {
  flex: 1;
  font-size: 15px;
}
.mvals > div {
  margin: 1px 0;
}
.over {
  color: var(--c-overdue);
  font-size: 14px;
}
</style>
