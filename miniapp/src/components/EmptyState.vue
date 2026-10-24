<script setup lang="ts">
defineProps<{ text: string; actionLabel?: string }>()
defineEmits<{ action: [] }>()
</script>

<template>
  <div class="empty">
    <p class="empty-text">{{ text }}</p>
    <button v-if="actionLabel" class="btn btn-ghost" type="button" @click="$emit('action')">
      {{ actionLabel }}
    </button>
  </div>
</template>

<style scoped>
.empty {
  text-align: center;
  padding: 32px 16px;
  color: var(--c-hint);
}
.empty-text {
  margin: 0 0 8px;
}
</style>
