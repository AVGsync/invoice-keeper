<!-- on/off toggle with a label -->
<script setup lang="ts">
const model = defineModel<boolean>({ default: false })
</script>

<template>
  <button type="button" role="switch" class="switch" :aria-checked="model" @click="model = !model">
    <span class="track" :class="{ on: model }"><span class="knob" /></span>
    <span class="label"><slot /></span>
  </button>
</template>

<style scoped>
.switch {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  color: var(--ink-2);
}

.track {
  position: relative;
  width: 28px;
  height: 16px;
  border-radius: 8px;
  background: var(--subtle);
}

.track.on {
  background: var(--accent);
}

.knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--ink);
  transition: left 0.14s ease;
}

.on .knob {
  left: 14px;
  background: #fff;
}
</style>
