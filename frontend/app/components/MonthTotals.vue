<script setup lang="ts">
import type { CurrencyTotal } from '~/utils/demo'

defineProps<{ title: string, totals: CurrencyTotal[] }>()
</script>

<template>
  <section class="card month">
    <h2 class="title display">{{ title }}</h2>
    <p class="kicker">По валютам · без конвертации</p>

    <div v-for="total in totals" :key="total.code" class="total">
      <UiLeaderRow>
        <template #label><span class="mono code">{{ total.code }}</span></template>
        <span class="sum">{{ total.total }}</span>
      </UiLeaderRow>
      <p class="paid mono">оплачено {{ total.paid }}</p>
    </div>
  </section>
</template>

<style scoped>
.month {
  view-transition-name: month-totals;
  padding: 12px 20px 18px;
}

.title {
  font-size: 23px;
}

.kicker {
  margin: 10px 0 15px;
}

.total + .total {
  margin-top: 8px;
}

.code,
.sum {
  font-weight: 500;
}

.paid {
  font-size: 11px;
  color: var(--ink-3);
  text-align: right;
}
</style>
