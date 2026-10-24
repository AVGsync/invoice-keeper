<script setup lang="ts">
withDefaults(defineProps<{ title: string, subtitle?: string }>(), { subtitle: '' })
</script>

<template>
  <header class="header">
    <div>
      <h1 class="title display">{{ title }}</h1>
      <p v-if="subtitle" class="subtitle">{{ subtitle }}</p>
    </div>
    <div class="actions">
      <slot name="actions" />
      <AppAskBar />
    </div>
  </header>
</template>

<style scoped>
.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding-top: 24px;
  margin-bottom: 24px;
}

.title {
  font-size: 24px;
  line-height: 32px;
}

.subtitle {
  margin-top: 2px;
  font-size: 14px;
  color: var(--ink-3);
}

.actions {
  display: flex;
  gap: 8px;
}
</style>
