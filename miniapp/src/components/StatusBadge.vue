<script setup lang="ts">
import { computed } from 'vue'
import { statusInfo } from '../lib/status'
import type { OccStatus } from '../api/types'

const props = defineProps<{ status: OccStatus; paidLate?: boolean }>()
const info = computed(() => statusInfo({ status: props.status, paid_late: props.paidLate }))
</script>

<template>
  <span class="badge" :style="{ color: `var(${info.colorVar})` }">
    <span class="dot" :style="{ background: `var(${info.colorVar})` }"></span>
    {{ info.label }}
  </span>
</template>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
</style>
