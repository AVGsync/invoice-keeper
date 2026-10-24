<script setup lang="ts">
import logo from '~/logo.svg'
</script>

<template>
  <span class="logo">
    <img :src="logo" class="mark" width="26" height="26" alt="">
    <span class="name">Invoice Keeper</span>
  </span>
</template>

<style scoped>
.logo {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.mark {
  flex-shrink: 0;
}

.name {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.02em;
}
</style>
