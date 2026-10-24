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
  color: var(--c-link);
  font-size: 16px;
  padding: 10px 0;
}
</style>
