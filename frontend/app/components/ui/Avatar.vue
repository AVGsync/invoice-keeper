<script setup lang="ts">
withDefaults(defineProps<{
  initials: string
  size?: number
  // blue avatar marks the current user
  me?: boolean
}>(), { size: 22 })
</script>

<template>
  <span
    class="avatar mono"
    :class="{ me }"
    :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.max(9, Math.round(size * 0.38))}px` }"
  >{{ initials }}</span>
</template>

<style scoped>
.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
  border-radius: 50%;
  /* letters scale with the circle (see fontSize above) and never spill out */
  line-height: 1;
  letter-spacing: -0.02em;
  background: var(--muted-bg);
  color: var(--ink-2);
  font-weight: 500;
}

.me {
  background: var(--accent);
  color: #fff;
}
</style>
