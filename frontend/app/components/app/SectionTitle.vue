<script setup lang="ts">
defineProps<{ index?: string, title: string, hint?: string }>()
</script>

<template>
  <div class="section">
    <h2 class="title display">{{ title }}</h2>
    <span v-if="hint" class="hint">{{ hint }}</span>
    <slot />
    <span class="right-side"><slot name="right" /></span>
  </div>
</template>

<style scoped>
.section {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title {
  font-size: 16px;
  line-height: 24px;
  letter-spacing: -0.01em;
}

/* sits on the title's baseline, not its middle */
.hint {
  margin-top: 2px;
  font-size: 13px;
  color: var(--ink-3);
}

.right-side {
  margin-left: auto;
}

/* phones */
@media (max-width: 1240px) {
  .section {
    flex-wrap: wrap;
    row-gap: 8px;
  }
}
</style>
