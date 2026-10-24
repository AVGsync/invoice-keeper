<script setup lang="ts">
import { computed } from 'vue'
import Money from './Money.vue'
import { nonZeroCurrencies, CURRENCY_ORDER } from '../lib/money'
import type { Currency } from '../api/types'

const props = defineProps<{
  totals: Record<Currency, number>
  inline?: boolean
}>()

const currencies = computed(() => {
  const nz = nonZeroCurrencies(props.totals)
  return nz.length ? nz : ([] as Currency[])
})
void CURRENCY_ORDER
</script>

<template>
  <span v-if="inline" class="inline">
    <template v-for="(c, i) in currencies" :key="c">
      <span v-if="i > 0"> · </span>
      <Money :amount="totals[c]" :currency="c" />
    </template>
  </span>
  <div v-else class="stacked">
    <div v-for="c in currencies" :key="c">
      <Money :amount="totals[c]" :currency="c" />
    </div>
  </div>
</template>

<style scoped>
.stacked {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
</style>
