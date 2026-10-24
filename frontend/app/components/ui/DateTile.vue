<script setup lang="ts">
defineProps<{ month: string, day: string }>()
</script>

<template>
  <span class="tile mono">
    <span class="month">{{ month }}</span>
    <span class="day">{{ day }}</span>
  </span>
</template>

<style scoped>
.tile {
  display: inline-flex;
  flex-direction: column;
  width: 42px;
  height: 46px;
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  text-align: center;
}

.month {
  height: 16px;
  padding-top: 4px;
  color: var(--ink-3);
  font-family: var(--sans);
  font-size: 10px;
  font-weight: 500;
  line-height: 12px;
}

.day {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 500;
}
</style>
