<script setup lang="ts">
withDefaults(defineProps<{ title: string, text: string, amount: string, canMark?: boolean }>(), { canMark: true })
defineEmits<{ mark: [] }>()
</script>

<template>
  <div class="banner">
    <span class="dot" />
    <strong class="title">{{ title }}</strong>
    <span class="text">{{ text }}</span>
    <span class="amount mono"><UiCountUp :value="amount" /></span>
    <UiButton v-if="canMark" @click="$emit('mark')">Отметить оплату</UiButton>
  </div>
</template>

<style scoped>
.banner {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 12px 0 18px;
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--red) 35%, var(--line));
  border-radius: 9999px;
  animation: banner-in 0.6s var(--ease-out) backwards;
}

/* comes down into place when there is something overdue */
@keyframes banner-in {
  from {
    opacity: 0;
    transform: translateY(-10px) scale(0.98);
  }
}

.dot {
  position: relative;
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--red);
}

/* a ring keeps spreading out of the dot, like an alarm light */
.dot::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: var(--red);
  animation: ping 2s var(--ease-out) infinite;
}

@keyframes ping {
  from {
    opacity: 0.6;
  }

  70%,
  to {
    opacity: 0;
    transform: scale(3.2);
  }
}

.title {
  font-weight: 600;
  color: var(--red);
}

.text {
  color: var(--ink-2);
}

.amount {
  margin: 0 10px 0 auto;
  font-size: 13px;
  font-weight: 500;
}
</style>
