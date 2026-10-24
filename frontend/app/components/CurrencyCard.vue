<script setup lang="ts">
import type { CurrencyTotal } from '~/utils/demo'

withDefaults(defineProps<{ total: CurrencyTotal, label?: string }>(), { label: 'К оплате за месяц¹' })
</script>

<template>
  <article class="card currency">
    <header class="head">
      <span class="code mono">{{ total.code }}</span>
      <span class="name">{{ total.name }}</span>
      <span class="count mono">{{ total.count }}</span>
    </header>

    <p class="label">{{ label }}</p>
    <p class="sum mono">{{ total.total }}</p>

    <div class="track">
      <div class="paid" :style="{ width: `${Math.max(total.progress * 100, 1.2)}%` }" />
    </div>

    <UiLeaderRow>
      <template #label>Оплачено</template>
      {{ total.paid }}
    </UiLeaderRow>
    <UiLeaderRow>
      <template #label>Не оплачено</template>
      {{ total.unpaid }}
    </UiLeaderRow>
    <UiLeaderRow>
      <template #label>Просрочено</template>
      <span :class="total.overdue ? 'overdue' : 'muted'">{{ total.overdue ?? '—' }}</span>
    </UiLeaderRow>
  </article>
</template>

<style scoped>
.currency {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 19px 22px 20px;
}

.head {
  display: flex;
  align-items: baseline;
  gap: 16px;
  margin-bottom: 14px;
}

.code {
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.06em;
}

.name {
  font-size: 13px;
  color: var(--ink-3);
}

.count {
  margin-left: auto;
  font-size: 12px;
  color: var(--ink-3);
}

.label {
  font-size: 12px;
  color: var(--ink-2);
}

.sum {
  font-size: 28px;
  font-weight: 500;
  letter-spacing: -0.02em;
}

.track {
  height: 4px;
  margin: 8px 0 14px;
  border-radius: 2px;
  background: var(--subtle);
}

.paid {
  height: 100%;
  border-radius: 2px;
  background: var(--accent);
}

.overdue {
  color: var(--red);
}
</style>
