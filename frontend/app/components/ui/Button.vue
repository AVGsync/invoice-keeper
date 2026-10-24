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
  border-radius: 6px;
  font-weight: 500;
  padding: 0 16px;
  white-space: nowrap;
}

.sm { height: 34px; font-size: 13px; }
.md { height: 36px; font-size: 13px; }
.lg { height: 42px; font-size: 14px; }

.primary {
  background: var(--accent);
  border: 0;
  color: #fff;
}

.secondary {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  color: var(--ink);
}

.block {
  width: 100%;
}
</style>
