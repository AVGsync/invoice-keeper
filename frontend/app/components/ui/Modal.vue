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
  background: rgb(20 20 20 / 0.28);
}

.modal {
  background: var(--surface);
  border-radius: 14px;
}

.head {
  display: flex;
  justify-content: space-between;
  padding: 18px 28px 0;
}

.title {
  font-size: 24px;
  line-height: 32px;
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
}

.body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 28px 24px;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 20px 28px;
  border-top: 1px solid var(--line);
}
</style>
