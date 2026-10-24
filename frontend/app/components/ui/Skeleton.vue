<!-- grey placeholder block shown where data will appear once it loads -->
<script setup lang="ts">
withDefaults(defineProps<{
  width?: string
  height?: string
  round?: boolean
}>(), { width: '100%', height: '12px' })
</script>

<template>
  <span class="skeleton" :class="{ round }" :style="{ width, height }" aria-hidden="true" />
</template>

<style scoped>
.skeleton {
  display: inline-block;
  vertical-align: middle;
  border-radius: 9999px;
  /* a soft light sweeps across, so it reads as «loading», not as an empty bar */
  background: linear-gradient(90deg, var(--subtle) 0%, var(--subtle) 35%, var(--muted-bg) 50%, var(--subtle) 65%, var(--subtle) 100%);
  background-size: 300% 100%;
  opacity: 0.8;
  animation: shimmer 1.6s ease-in-out infinite;
}

@keyframes shimmer {
  from {
    background-position: 100% 0;
  }

  to {
    background-position: 0 0;
  }
}

.round {
  border-radius: 50%;
}
</style>
