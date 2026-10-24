<script setup lang="ts">
import type { Period } from '~/utils/periods'

// canMark: false hides the button (somebody else's payment, for a user)
// others: the rest of that day's payments, a click shows one of them here
withDefaults(defineProps<{ kicker: string, title: string, period: Period, others?: Period[], canMark?: boolean }>(), { canMark: true, others: () => [] })
defineEmits<{ mark: [], pick: [period: Period] }>()
</script>

<template>
  <section class="day">
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

    <UiStatus
      :status="period.status"
      :label="period.paidAt ? `Оплачено ${period.paidAt}` : period.reminded ? `Ожидает · напомнили ${period.reminded}` : undefined"
    />

    <UiButton v-if="!period.paidAt && period.status !== 'ended' && canMark" variant="primary" size="lg" block @click="$emit('mark')">
      Отметить оплату
    </UiButton>

    <div v-if="others.length" class="others">
      <p class="kicker">Ещё в этот день</p>
      <button v-for="other in others" :key="other.id" type="button" class="other" @click="$emit('pick', other)">
        <span class="dot" :class="other.status" />
        <span class="other-name">{{ other.title }}</span>
        <span class="mono">{{ other.amount }}</span>
      </button>
    </div>
  </section>
</template>

<style scoped>
/* the lines come up one after another when a payment is picked */
.day > * {
  animation: rise 0.5s var(--ease-out) backwards;
}

.day > :nth-child(2) { animation-delay: 40ms; }
.day > :nth-child(3) { animation-delay: 80ms; }
.day > :nth-child(4) { animation-delay: 120ms; }
.day > :nth-child(5) { animation-delay: 160ms; }
.day > :nth-child(6) { animation-delay: 200ms; }
.day > :nth-child(n + 7) { animation-delay: 240ms; }

.day {
  view-transition-name: day-panel;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0 20px;
  border-bottom: 1px solid var(--line);
}

.others {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-top: 14px;
  border-top: 1px solid var(--line);
}

.others .kicker {
  margin-bottom: 6px;
}

.other {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  margin: 0 -8px;
  padding: 0 8px;
  background: none;
  border: 0;
  border-radius: 9999px;
  font-size: 13px;
  text-align: left;
  transition: background-color 0.2s ease;
}

.other:hover {
  background: var(--subtle);
}

.other-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.dot.pending { background: var(--accent-ink); }
.dot.paid { background: var(--green); }
.dot.late { background: var(--amber); }
.dot.overdue { background: var(--red); }
.dot.ended { background: var(--ink-3); }

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
