<!-- message in place of a list: nothing there yet, or loading failed -->
<script setup lang="ts">
withDefaults(defineProps<{
  kind?: 'empty' | 'error'
}>(), { kind: 'empty' })

defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="state" :class="kind" role="status">
    <p class="text"><slot /></p>
    <UiButton v-if="kind === 'error'" @click="$emit('retry')">Повторить</UiButton>
  </div>
</template>

<style scoped>
.state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 32px 20px;
  text-align: center;
}

.text {
  font-size: 13px;
  color: var(--ink-3);
}

.error .text {
  color: var(--red);
}
</style>
