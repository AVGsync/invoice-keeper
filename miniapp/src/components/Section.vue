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
  margin: 16px 0;
}
.section-header {
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.02em;
  color: var(--c-section-header);
  margin: 0 0 8px 4px;
}
.section-body {
  background: var(--c-section);
  border-radius: var(--radius);
  overflow: hidden;
}
.section-footer {
  color: var(--c-hint);
  font-size: 13px;
  margin: 8px 4px 0;
}
</style>
