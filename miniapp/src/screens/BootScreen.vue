<script setup lang="ts">
import Skeleton from '../components/Skeleton.vue'
</script>

<template>
  <div class="boot">
    <h1 class="title">Invoice Keeper</h1>
    <Skeleton :rows="5" />
  </div>
</template>

<style scoped>
.boot {
  padding-top: 24px;
}
.title {
  font-size: 22px;
  margin-bottom: 16px;
}
</style>
