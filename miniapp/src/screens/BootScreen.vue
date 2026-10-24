<script setup lang="ts">
import Skeleton from '../components/Skeleton.vue'
import Prism from '../components/Prism.vue'
</script>

<template>
  <div class="boot">
    <Prism size="hero" class="boot-prism" />
    <h1 class="title boot-title">Invoice Keeper</h1>
    <Skeleton :rows="5" class="boot-sk" />
  </div>
</template>

<style scoped>
.boot {
  min-height: 70vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--sp-6);
}
.boot-prism {
  --prism-size: 160px;
}
.boot-title {
  font-size: clamp(32px, 10vw, 40px);
  text-align: center;
  margin: 0;
}
.boot-sk {
  align-self: stretch;
}
</style>
