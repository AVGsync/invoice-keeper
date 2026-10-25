<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  subtitle?: string
  width?: number
}>(), { width: 480, subtitle: '' })

defineEmits<{ close: [] }>()
</script>

<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal" :style="{ width: `${width}px` }">
      <header class="head">
        <div>
          <h2 class="title display">{{ title }}</h2>
          <p v-if="subtitle" class="subtitle">{{ subtitle }}</p>
        </div>
        <button type="button" class="close" @click="$emit('close')">×</button>
      </header>

      <div class="body">
        <slot />
      </div>

      <footer class="foot">
        <slot name="footer" />
      </footer>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(0 0 0 / 0.4);
  backdrop-filter: blur(6px);
}

.modal {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
  box-shadow: var(--shadow-pop);
}

.head {
  display: flex;
  justify-content: space-between;
  padding: 20px 24px 0;
}

.title {
  font-size: 18px;
  line-height: 28px;
}

.subtitle {
  margin-top: 6px;
  font-size: 13px;
  color: var(--ink-3);
}

.close {
  align-self: flex-start;
  margin-top: 6px;
  background: none;
  border: 0;
  font-size: 22px;
  line-height: 1;
  color: var(--ink-3);
  transition: color 0.2s ease, transform 0.4s var(--ease-spring);
}

.close:hover {
  color: var(--ink);
  transform: rotate(90deg);
}

/* opening: the fields come up one after another after the window, the buttons last */
.body > *,
.foot {
  animation: rise 0.5s var(--ease-out) backwards;
  animation-delay: 80ms;
}

.body > :nth-child(2) { animation-delay: 120ms; }
.body > :nth-child(3) { animation-delay: 160ms; }
.body > :nth-child(4) { animation-delay: 200ms; }
.body > :nth-child(5) { animation-delay: 240ms; }
.body > :nth-child(n + 6) { animation-delay: 280ms; }

.foot {
  animation-delay: 300ms;
}

.body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 24px 24px;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 24px;
  border-top: 1px solid var(--line);
}

/* phones */
@media (max-width: 820px) {
  /* a sheet from the bottom edge, as wide as the screen */
  .overlay {
    align-items: flex-end;
  }

  .modal {
    display: flex;
    flex-direction: column;
    width: 100% !important;
    max-height: 92dvh;
    border-bottom: 0;
    border-radius: 20px 20px 0 0;
  }

  .head {
    padding: 18px 16px 0;
  }

  .close {
    width: 44px;
    height: 44px;
    margin: -8px -10px 0 0;
    font-size: 26px;
  }

  .body {
    flex: 1 1 auto;
    min-height: 0;
    padding: 14px 16px 16px;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .foot {
    padding: 12px 16px calc(12px + env(safe-area-inset-bottom, 0px));
  }

  .foot > * {
    flex: 1 1 0;
    height: 44px;
  }
}
</style>
