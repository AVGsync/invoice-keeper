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
  height: 48px;
  border-radius: 10px;
  margin: 8px 0;
  background: linear-gradient(
    90deg,
    var(--c-secondary-bg) 25%,
    var(--c-section) 37%,
    var(--c-secondary-bg) 63%
  );
  background-size: 400% 100%;
  animation: sk 1.4s ease infinite;
}
@keyframes sk {
  0% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0 50%;
  }
}
@media (prefers-reduced-motion: reduce) {
  .sk-row {
    animation: none;
  }
}
</style>
