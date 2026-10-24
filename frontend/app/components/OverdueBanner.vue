<script setup lang="ts">
defineProps<{ title: string, text: string, amount: string, soon?: boolean }>()
defineEmits<{ mark: [] }>()
</script>

<template>
  <div class="banner">
    <span class="dot" />
    <strong class="title">{{ title }}</strong>
    <span class="text">{{ text }}</span>
    <span class="amount mono">{{ amount }}</span>
    <UiButton :soon="soon" @click="$emit('mark')">Отметить оплату</UiButton>
  </div>
</template>

<style scoped>
.banner {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 52px;
  padding: 0 14px 0 18px;
  background: var(--red-bg);
  border-radius: 8px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--red);
}

.title {
  font-weight: 500;
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
