<script setup lang="ts">
import { computed } from 'vue'
import { formatMoney } from '../lib/money'
import type { Currency } from '../api/types'

const props = defineProps<{ amount: number; currency: Currency; large?: boolean }>()
const text = computed(() => formatMoney(props.amount, props.currency))
</script>

<template>
  <span class="num mono-amount" :class="{ large }">{{ text }}</span>
</template>

<style scoped>
.large {
  font-size: 28px;
  font-weight: 700;
}
</style>
