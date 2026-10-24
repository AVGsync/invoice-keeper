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
  margin: var(--sp-8) 0 0;
}
.section-header {
  font-size: var(--fs-caption);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--fog);
  line-height: 1.2;
  margin: 0 0 var(--sp-3) 0;
}
.section-body {
  background: transparent;
  border-radius: 0;
  overflow: visible;
}
.section-header + .section-body {
  border-top: 1px solid var(--c-line);
}
.section:not(:has(.section-header))
  > .section-body:has(> :is(.cell, .occ-row, .rrow, .urow, .radio-row):first-child) {
  border-top: 1px solid var(--c-line);
}
.section-footer {
  color: var(--fog);
  font-size: var(--fs-small);
  margin: var(--sp-3) 0 0;
}
</style>
