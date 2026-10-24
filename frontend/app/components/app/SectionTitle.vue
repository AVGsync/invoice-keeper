<script setup lang="ts">
defineProps<{ index?: string, title: string, hint?: string }>()
</script>

<template>
  <div class="section">
    <span v-if="index" class="index mono">{{ index }}</span>
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

.index {
  font-size: 12px;
  font-weight: 500;
  color: var(--accent-ink);
}

.title {
  font-size: 23px;
  line-height: 30px;
}

/* sits on the title's baseline, not its middle */
.hint {
  margin: 6px 0 0 6px;
  font-size: 13px;
  color: var(--ink-3);
}

.right-side {
  margin-left: auto;
}
</style>
