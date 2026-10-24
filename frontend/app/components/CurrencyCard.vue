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
  gap: 6px;
  padding: 20px;
}

.head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 12px;
}

.code {
  padding: 1px 8px;
  border: 1px solid var(--line);
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
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
  font-size: 13px;
  color: var(--ink-3);
}

.sum {
  font-size: 26px;
  font-weight: 500;
  line-height: 34px;
  letter-spacing: -0.03em;
}

.track {
  height: 4px;
  margin: 6px 0 14px;
  border-radius: 9999px;
  background: var(--subtle);
}

.paid {
  height: 100%;
  border-radius: 9999px;
  background: var(--accent);
  transform-origin: left;
  animation: fill 1.1s var(--ease-out) 0.25s backwards;
  transition: width 0.6s var(--ease-out);
}

@keyframes fill {
  from { transform: scaleX(0); }
}

.currency {
  transition: border-color 0.25s ease, transform 0.35s var(--ease-out);
}

.currency:hover {
  border-color: var(--line-strong);
  transform: translateY(-2px);
}

.overdue {
  color: var(--red);
}
</style>
