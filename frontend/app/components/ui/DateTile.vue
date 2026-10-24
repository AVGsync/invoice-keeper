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
  border-radius: 6px;
  text-align: center;
}

.month {
  height: 14px;
  background: var(--accent);
  color: #fff;
  font-size: 8px;
  font-weight: 600;
  letter-spacing: 0.1em;
  line-height: 14px;
}

.day {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 500;
  letter-spacing: -0.02em;
}
</style>
