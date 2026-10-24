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
  background: var(--c-section);
  border-radius: var(--radius);
  overflow: hidden;
  margin: 8px 0;
}
.block-title {
  padding: 10px 14px 4px;
  color: var(--c-hint);
  font-size: 13px;
}
.crow {
  display: flex;
  gap: 12px;
  padding: 10px 14px;
  border-top: 1px solid var(--c-secondary-bg);
}
.cur {
  font-weight: 600;
  width: 48px;
}
.vals {
  flex: 1;
}
.total {
  font-weight: 600;
}
.sub {
  margin-top: 2px;
}
</style>
