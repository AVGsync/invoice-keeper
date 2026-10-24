<script setup lang="ts">
withDefaults(defineProps<{ rows?: number }>(), { rows: 4 })
</script>

<template>
  <div class="skeleton">
    <div v-for="n in rows" :key="n" class="sk-row"></div>
  </div>
</template>

<style scoped>
.skeleton {
  padding: 8px 0;
}
.sk-row {
  position: relative;
  overflow: hidden;
  height: 56px;
  border-radius: 16px;
  margin: 0 0 8px;
  background: var(--graphite);
}
/* light passes across the rows, slightly offset per row */
.sk-row::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, var(--shimmer) 50%, transparent);
  transform: translateX(-100%);
  animation: shimmer 1.5s linear infinite;
}
.sk-row:nth-child(2n)::after {
  animation-delay: 0.12s;
}
.sk-row:nth-child(3n)::after {
  animation-delay: 0.24s;
}
@keyframes shimmer {
  to {
    transform: translateX(100%);
  }
}
@media (prefers-reduced-motion: reduce) {
  .sk-row::after {
    display: none;
  }
}
</style>
