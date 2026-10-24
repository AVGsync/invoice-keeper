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
  height: 56px;
  border-radius: 0;
  margin: 0;
  background: var(--veil-weak);
  border-bottom: 1px solid var(--obsidian);
  animation: sk 1.6s var(--ease) infinite alternate;
}
.sk-row:nth-child(2n) {
  animation-delay: 0.2s;
}
@keyframes sk {
  from {
    opacity: 0.45;
  }
  to {
    opacity: 1;
  }
}
@media (prefers-reduced-motion: reduce) {
  .sk-row {
    animation: none;
  }
}
</style>
