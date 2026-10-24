<script setup lang="ts">
withDefaults(defineProps<{
  variant?: 'primary' | 'secondary'
  size?: 'sm' | 'md' | 'lg'
  block?: boolean
  // not working yet, waits for the backend: amber «скоро» look, disabled
  soon?: boolean
}>(), { variant: 'secondary', size: 'sm' })
</script>

<template>
  <button
    class="btn"
    :class="[variant, size, { block, soon }]"
    :disabled="soon || undefined"
    :title="soon ? soonHint : undefined"
  >
    <slot />
  </button>
</template>

<style scoped>
.btn {
  border-radius: 9999px;
  font-weight: 500;
  padding: 0 16px;
  white-space: nowrap;
  transition: background-color 0.14s ease, border-color 0.14s ease, opacity 0.14s ease, transform 0.2s var(--ease-spring);
}

.sm { height: 34px; font-size: 13px; }
.md { height: 36px; font-size: 13px; }
.lg { height: 42px; font-size: 14px; }

/* near-black (white in the dark theme), once per screen */
.primary {
  background: var(--primary);
  border: 0;
  color: var(--on-primary);
  box-shadow: var(--shadow-button);
}

.primary:hover:not(:disabled) {
  opacity: 0.86;
}

.secondary {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  color: var(--ink);
}

.secondary:hover:not(:disabled) {
  background: var(--subtle);
}

.block {
  width: 100%;
}
</style>
