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
  text-align: left;
  padding: var(--sp-8) 0;
  color: var(--fog);
}
.empty-text {
  font-size: var(--fs-body-l);
  margin: 0 0 var(--sp-3);
}
</style>
