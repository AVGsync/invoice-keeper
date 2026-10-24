<script setup lang="ts">
import logo from '~/logo.svg'
</script>

<template>
  <span class="logo">
    <img :src="logo" class="mark" width="18" height="18" alt="">
    <span class="name">Invoice Keeper</span>
  </span>
</template>

<style scoped>
.logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.mark {
  flex-shrink: 0;
}

.name {
  font-family: var(--display);
  font-size: 14px;
  font-weight: 600;
}
</style>
