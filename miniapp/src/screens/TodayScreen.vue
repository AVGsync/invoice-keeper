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
import Horizon from '../components/Horizon.vue'
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
    <div class="title-row">
      <h1 class="title">Сегодня</h1>
      <Horizon class="mark" size="mark" />
    </div>
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
        <span class="ov-bg" aria-hidden="true" />
        <span class="ov-sheen" aria-hidden="true" />
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
.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sp-3);
  margin: var(--sp-6) 0 var(--sp-5);
}
.title-row .title {
  margin: 0;
}
/* Hero Horizon — the one full-colour surface in the app */
.overdue-card {
  display: block;
  width: 100%;
  text-align: left;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: var(--radius);
  background: #5b4bd0;
  color: var(--on-horizon);
  padding: var(--sp-5) var(--sp-5) var(--sp-6);
  margin: var(--sp-5) 0 0;
  position: relative;
  overflow: hidden;
  isolation: isolate;
  transition: transform var(--dur-press) var(--ease-press);
}
/* soft vignette keeps small white text ≥4.5:1 on the warm end */
.overdue-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.18), rgba(0, 0, 0, 0) 60%);
  z-index: -1;
}
/* the gradient layer is wider than the card and drifts slowly — transform only */
.ov-bg {
  position: absolute;
  inset: 0 -40%;
  background: var(--horizon);
  z-index: -2;
  animation: drift 14s var(--ease-in-out) infinite alternate;
}
@keyframes drift {
  from {
    transform: translateX(-12%);
  }
  to {
    transform: translateX(12%);
  }
}
/* one light sweep when the card appears */
.ov-sheen {
  position: absolute;
  inset: -20% auto -20% 0;
  width: 45%;
  background: linear-gradient(
    100deg,
    transparent,
    rgba(255, 255, 255, 0.22) 50%,
    transparent
  );
  transform: translateX(-120%) skewX(-12deg);
  animation: sweep 1.1s var(--ease-out) 0.45s both;
  pointer-events: none;
}
@keyframes sweep {
  to {
    transform: translateX(320%) skewX(-12deg);
  }
}
.ov-head,
.ov-sum {
  position: relative;
}
@media (prefers-reduced-motion: reduce) {
  .ov-bg,
  .ov-sheen {
    animation: none;
  }
  .ov-sheen {
    display: none;
  }
}
.overdue-card:active {
  transform: scale(0.985);
}
.overdue-card::after {
  content: '';
  position: absolute;
  top: var(--sp-5);
  right: var(--sp-5);
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background:
    no-repeat center / 14px
      url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 14 14'%3E%3Cpath d='M1 7h11M8 3l4 4-4 4' fill='none' stroke='%23161616' stroke-width='1.6' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E"),
    var(--on-horizon);
}
.ov-head {
  font-size: var(--fs-small);
  font-weight: 500;
  opacity: 0.92;
  margin-bottom: var(--sp-5);
}
.ov-sum {
  font-size: var(--fs-amount-l);
  line-height: 1.05;
  letter-spacing: -0.035em;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.empty-inline {
  padding: var(--sp-4) 0;
  color: var(--muted);
  margin: 0;
}
.month {
  padding: 0;
}
.mrow {
  display: flex;
  gap: var(--sp-4);
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.cur {
  width: 40px;
  padding-top: 2px;
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--muted);
}
.mvals {
  flex: 1;
  font-size: var(--fs-body-l);
}
.mvals > div {
  margin: 2px 0;
}
.mvals > div :deep(.num) {
  float: right;
}
.over {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--bone-text);
  font-size: var(--fs-small);
}
.over::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--red);
  flex: 0 0 6px;
}
.over :deep(.num) {
  margin-left: auto;
}
</style>
