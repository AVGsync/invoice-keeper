<script setup lang="ts">
import { computed } from 'vue'
import { CURRENCY_ORDER, formatMoney } from '../../lib/money'
import type { CurrencyTotalsBlock, Currency } from '../../api/types'

const props = defineProps<{ block: CurrencyTotalsBlock }>()

// Агент отдаёт суммы в старших единицах строкой ("150000.00"). Переводим в
// минорные и форматируем через formatMoney (как везде в приложении).
function toMinor(v: number | string | undefined): number | null {
  if (v === undefined || v === null || v === '') return null
  const n = typeof v === 'number' ? v : parseFloat(v)
  if (!Number.isFinite(n)) return null
  return Math.round(n * 100)
}

const rows = computed(() =>
  CURRENCY_ORDER.filter((c) => props.block.by_currency[c as Currency]).map((c) => {
    const currency = c as Currency
    const entry = props.block.by_currency[currency]!
    const paid = toMinor(entry.paid)
    const unpaid = toMinor(entry.unpaid)
    // Подстрока «Оплачено · Осталось» — только когда обе величины есть
    // (у basis=run_rate полей paid/unpaid нет).
    const sub =
      paid !== null && unpaid !== null
        ? `Оплачено ${formatMoney(paid, currency)} · Осталось ${formatMoney(unpaid, currency)}`
        : null
    return { currency, entry, sub }
  }),
)
</script>

<template>
  <div class="block">
    <div v-if="block.title" class="block-title">{{ block.title }}</div>
    <div v-for="r in rows" :key="r.currency" class="crow">
      <div class="cur">{{ r.currency }}</div>
      <div class="vals">
        <div class="total num mono-amount">{{ r.entry.display }}</div>
        <div v-if="r.sub" class="sub hint">{{ r.sub }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.block {
  background: var(--graphite);
  border: 1px solid var(--line);
  border-radius: 20px;
  overflow: hidden;
  margin: 0;
}
.block-title {
  padding: var(--sp-4) var(--sp-5) var(--sp-2);
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--fog);
  line-height: 1.2;
}
.crow {
  display: flex;
  gap: 12px;
  padding: 14px var(--sp-5);
  border-top: 1px solid var(--c-line);
}
.cur {
  width: 48px;
  font-size: var(--fs-caption);
  color: var(--fog);
}
.vals {
  flex: 1;
}
.total {
  font-size: var(--fs-title);
  font-weight: 400;
}
.sub {
  margin-top: 4px;
}
</style>
