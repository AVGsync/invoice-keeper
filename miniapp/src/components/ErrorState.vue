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
  text-align: left;
  padding: var(--sp-8) 0;
}
.err-text {
  color: var(--bone);
  font-size: var(--fs-body-l);
  border-left: 2px solid var(--bone);
  padding-left: var(--sp-3);
  margin: 0 0 var(--sp-4);
}
</style>
