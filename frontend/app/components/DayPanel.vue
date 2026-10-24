<script setup lang="ts">
import type { Period } from '~/utils/demo'

defineProps<{ kicker: string, title: string, period: Period, soon?: boolean }>()
defineEmits<{ mark: [] }>()
</script>

<template>
  <section class="card day">
    <p class="kicker accent">{{ kicker }}</p>
    <h2 class="title display">{{ title }}</h2>

    <div class="row">
      <span>
        <span class="name">{{ period.title }}</span>
        <span class="meta">{{ period.meta }}</span>
      </span>
      <span class="amount mono">{{ period.amount }}</span>
    </div>

    <div class="row">
      <PersonCell :person="period.who" />
      <span class="persona mono">{{ period.who.persona }}</span>
    </div>

    <UiStatus :status="period.status" :label="period.reminded ? `Ожидает · напомнили ${period.reminded}` : undefined" />

    <UiButton v-if="!period.paidAt" variant="primary" size="lg" block :soon="soon" @click="$emit('mark')">
      Отметить оплату
    </UiButton>
  </section>
</template>

<style scoped>
.day {
  view-transition-name: day-panel;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px 20px 20px;
}

.accent {
  color: var(--accent-ink);
}

.title {
  margin: -12px 0 -2px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--line);
  font-size: 20px;
  line-height: 28px;
}

.row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.name {
  display: block;
  font-size: 14px;
  font-weight: 500;
}

.meta {
  font-size: 12px;
  color: var(--ink-3);
}

.amount {
  font-size: 14px;
  font-weight: 500;
}

.persona {
  font-size: 12px;
  color: var(--ink-3);
}
</style>
