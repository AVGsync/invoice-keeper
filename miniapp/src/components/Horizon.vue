<script setup lang="ts">
// Знак бренда в стиле Dimension: «закат» amber → violet → cobalt, мягко светится.
withDefaults(defineProps<{ size?: 'mark' | 'hero' }>(), { size: 'hero' })
</script>

<template>
  <span class="horizon" :class="size" aria-hidden="true">
    <span class="glow" />
    <span class="disc" />
  </span>
</template>

<style scoped>
.horizon {
  position: relative;
  display: block;
  flex: 0 0 auto;
}
.mark {
  width: 28px;
  height: 28px;
}
.hero {
  width: var(--horizon-size, 120px);
  height: var(--horizon-size, 120px);
  animation: bloom 0.9s var(--ease-out) both;
}
/* rare screens only (boot / link / error): the orb blooms in */
@keyframes bloom {
  from {
    opacity: 0;
    transform: scale(0.88);
    filter: blur(8px);
  }
}
.disc,
.glow {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background:
    radial-gradient(circle at 28% 26%, rgba(255, 214, 170, 0.75), transparent 42%),
    linear-gradient(140deg, #e0703a 0%, #b44a7a 38%, #6b62f2 68%, #2f5bd8 100%);
}
.disc {
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.18);
}
.glow {
  filter: blur(18px);
  opacity: 0.55;
  transform: scale(1.15);
  animation: breathe 6s var(--ease) infinite alternate;
}
.mark .glow {
  filter: blur(6px);
  opacity: 0.5;
}
@keyframes breathe {
  to {
    opacity: 0.8;
    transform: scale(1.3);
  }
}
@media (prefers-reduced-motion: reduce) {
  .glow {
    animation: none;
  }
}
</style>
