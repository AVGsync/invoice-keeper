<script setup lang="ts">
defineProps<{ title?: string; footer?: string }>()
</script>

<template>
  <section class="section">
    <h2 v-if="title" class="section-header">{{ title }}</h2>
    <div class="section-body">
      <slot />
    </div>
    <p v-if="footer" class="section-footer">{{ footer }}</p>
  </section>
</template>

<style scoped>
.section {
  margin: var(--sp-7) 0 0;
}
.section-header {
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--muted);
  line-height: 1.3;
  margin: 0 0 var(--sp-2) var(--sp-1);
}
.section-body {
  background: var(--graphite);
  border: 1px solid var(--line);
  border-radius: 20px;
  padding: 0 var(--sp-4);
  overflow: hidden;
}
.section-body :deep(input:not([type='radio']):not([type='checkbox'])) {
  background: transparent;
  border: 0;
  border-radius: 0;
  padding-left: 0;
  padding-right: 0;
}
.section-body:empty {
  display: none;
}
.section-body > :last-child,
.section-body > :last-child > :last-child {
  border-bottom: 0 !important;
}
.section-footer {
  color: var(--muted);
  font-size: var(--fs-small);
  margin: var(--sp-2) var(--sp-1) 0;
}
</style>
