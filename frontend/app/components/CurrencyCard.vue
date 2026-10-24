<script setup lang="ts">
import type { CurrencyTotal } from '~/utils/periods'

// loading: grey blocks instead of the numbers, so the card doesn't show zeros before the data arrives
withDefaults(defineProps<{ total: CurrencyTotal, label?: string, loading?: boolean }>(), { label: 'К оплате за месяц¹', loading: false })
</script>

<template>
  <article class="card currency">
    <header class="head">
      <span class="code mono">{{ total.code }}</span>
      <span class="name">{{ total.name }}</span>
      <UiSkeleton v-if="loading" width="70px" height="11px" class="count" />
      <span v-else class="count mono">{{ total.count }}</span>
    </header>

    <p class="label">{{ label }}</p>
    <p v-if="loading" class="sum"><UiSkeleton width="190px" height="26px" /></p>
    <p v-else class="sum mono"><UiCountUp :value="total.total" /></p>

    <div class="track">
      <div v-if="!loading" class="paid" :style="{ width: `${Math.max(total.progress * 100, 1.2)}%` }" />
    </div>

    <UiLeaderRow>
      <template #label>Оплачено</template>
      <UiSkeleton v-if="loading" width="80px" />
      <UiCountUp v-else :value="total.paid" />
    </UiLeaderRow>
    <UiLeaderRow>
      <template #label>Не оплачено</template>
      <UiSkeleton v-if="loading" width="70px" />
      <UiCountUp v-else :value="total.unpaid" />
    </UiLeaderRow>
    <UiLeaderRow>
      <template #label>Просрочено</template>
      <UiSkeleton v-if="loading" width="60px" />
      <span v-else :class="total.overdue ? 'overdue' : 'muted'"><UiCountUp v-if="total.overdue" :value="total.overdue" /><template v-else>—</template></span>
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
