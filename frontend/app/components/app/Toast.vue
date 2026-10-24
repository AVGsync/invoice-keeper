<!-- a failed payment action (usePayments.actionError) shows up here for a few seconds -->
<script setup lang="ts">
const { actionError } = usePayments()

let timer: ReturnType<typeof setTimeout> | undefined
watch(actionError, (text) => {
  clearTimeout(timer)
  if (text) timer = setTimeout(() => (actionError.value = ''), 6000)
})
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <Transition name="toast">
    <div v-if="actionError" class="toast" role="alert">
      <span class="dot" />
      <span class="text">{{ actionError }}</span>
      <button type="button" class="close" aria-label="Закрыть" @click="actionError = ''">×</button>
    </div>
  </Transition>
</template>

<style scoped>
.toast {
  position: fixed;
  right: 24px;
  bottom: calc(24px + env(safe-area-inset-bottom, 0px));
  z-index: 30;
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 420px;
  padding: 12px 12px 12px 16px;
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--red) 35%, var(--line));
  border-radius: 9999px;
  box-shadow: var(--shadow-pop);
  font-size: 13px;
}

.dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--red);
}

.text {
  flex: 1;
}

.close {
  padding: 0 4px;
  background: none;
  border: 0;
  font-size: 18px;
  line-height: 1;
  color: var(--ink-3);
}

.close:hover {
  color: var(--ink);
}

.toast-enter-active {
  transition: opacity 0.2s ease, transform 0.45s var(--ease-out);
}

.toast-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(12px) scale(0.97);
}
</style>
