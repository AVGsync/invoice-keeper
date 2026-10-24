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
  border-radius: 4px;
  background: var(--subtle);
  opacity: 0.7;
}

.round {
  border-radius: 50%;
}
</style>
