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
/* a hairline under the title on every page, so it stands apart from what comes next */
.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 24px 0 14px;
  margin-bottom: 18px;
  border-bottom: 1px solid var(--line);
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

/* phones: the title on top, buttons and the question bar under it, full width */
@media (max-width: 1240px) {
  .header {
    flex-direction: column;
    gap: 12px;
    padding: 18px 0 14px;
    margin-bottom: 14px;
  }

  .title {
    font-size: 22px;
    line-height: 28px;
  }

  .actions {
    flex-wrap: wrap;
    width: 100%;
  }
}
</style>
