<script setup lang="ts">
defineEmits<{ back: [] }>()
</script>

<template>
  <button class="inline-back" type="button" aria-label="Назад" @click="$emit('back')">
    ‹ Назад
  </button>
</template>

<style scoped>
.inline-back {
  border: 0;
  background: transparent;
  color: var(--ash-text);
  font-size: var(--fs-body);
  font-weight: 500;
  min-height: 44px;
  padding: 0;
}
</style>
