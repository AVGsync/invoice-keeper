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
  animation: state-in 0.5s var(--ease-out) backwards;
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
  animation: no 0.5s ease 0.15s;
}

/* comes up softly in place of the list */
@keyframes state-in {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
}

/* an error: a small shake, like shaking the head */
@keyframes no {
  25% {
    transform: translateX(-4px);
  }

  50% {
    transform: translateX(4px);
  }

  75% {
    transform: translateX(-2px);
  }
}
</style>
