<script setup lang="ts">
import Skeleton from '../components/Skeleton.vue'
import Horizon from '../components/Horizon.vue'
</script>

<template>
  <div class="boot">
    <Horizon size="hero" class="boot-horizon" />
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
.boot-horizon {
  --horizon-size: 112px;
}
.boot-title {
  text-align: center;
  margin: 0;
}
.boot-sk {
  align-self: stretch;
}
</style>
