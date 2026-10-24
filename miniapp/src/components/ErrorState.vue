<script setup lang="ts">
import { humanError } from '../lib/humanError'

const props = defineProps<{ error: unknown }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="err">
    <p class="err-text">{{ humanError(props.error) }}</p>
    <button class="btn btn-secondary" type="button" @click="$emit('retry')">Повторить</button>
  </div>
</template>

<style scoped>
.err {
  text-align: center;
  padding: 32px 16px;
}
.err-text {
  color: var(--c-destructive);
  margin: 0 0 12px;
}
</style>
