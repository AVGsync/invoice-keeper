<script setup lang="ts">
import { computed } from 'vue'
import { formatMoney } from '../lib/money'
import type { Currency } from '../api/types'

const props = defineProps<{ amount: number; currency: Currency; large?: boolean }>()
const text = computed(() => formatMoney(props.amount, props.currency))
const long = computed(() => text.value.length > 10)
</script>

<template>
  <span class="num mono-amount" :class="{ large, long }">{{ text }}</span>
</template>

<style scoped>
.large {
  font-size: var(--fs-amount-xl);
  line-height: 1;
  letter-spacing: -0.02em;
  font-weight: 400;
}
.large.long {
  font-size: 36px;
}
</style>
