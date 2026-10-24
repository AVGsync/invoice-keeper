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
  color: var(--bone);
  font-size: var(--fs-caption);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  min-height: 44px;
  padding: 0;
}
</style>
