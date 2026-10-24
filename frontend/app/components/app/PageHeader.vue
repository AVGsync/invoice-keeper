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
  padding-top: 14px;
  margin-bottom: 19px;
}

.title {
  margin-left: -2px;
  font-size: 38px;
  line-height: 50px;
  letter-spacing: -0.01em;
}

.subtitle {
  font-size: 13px;
  color: var(--ink-3);
}

.actions {
  display: flex;
  gap: 12px;
  margin-top: 12px;
}
</style>
