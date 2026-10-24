<script setup lang="ts">
withDefaults(defineProps<{ title: string, text: string, amount: string, canMark?: boolean }>(), { canMark: true })
defineEmits<{ mark: [] }>()
</script>

<template>
  <div class="banner">
    <span class="dot" />
    <strong class="title">{{ title }}</strong>
    <span class="text">{{ text }}</span>
    <span class="amount mono">{{ amount }}</span>
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
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--red);
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
