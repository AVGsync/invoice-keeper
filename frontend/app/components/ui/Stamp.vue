<script setup lang="ts">
withDefaults(defineProps<{
  color?: 'green' | 'red' | 'accent'
  tilt?: number
  large?: boolean
}>(), { color: 'green', tilt: 0 })
</script>

<template>
  <span class="stamp mono" :class="[color, { large }]" :style="{ transform: `rotate(${tilt}deg)` }">
    <slot />
  </span>
</template>

<style scoped>
.stamp {
  display: inline-block;
  padding: 6px 12px;
  border: 1px solid currentColor;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.08em;
  white-space: nowrap;
  transform-origin: top left;
}

.large {
  padding: 12px 16px;
}

.green { color: var(--green); }
.red { color: var(--red); }
.accent { color: var(--accent-ink); }
</style>
