<script setup lang="ts">
import type { CurrencyTotal } from '~/utils/demo'

defineProps<{ title: string, totals: CurrencyTotal[] }>()
</script>

<template>
  <section class="month">
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
  padding: 16px 0 18px;
  border-bottom: 1px solid var(--line);
}

.title {
  font-size: 16px;
  line-height: 24px;
}

.kicker {
  margin: 4px 0 14px;
}

.total + .total {
  margin-top: 8px;
}

.code,
.sum {
  font-weight: 500;
}

.paid {
  font-size: 12px;
  color: var(--ink-3);
  text-align: right;
}
</style>
