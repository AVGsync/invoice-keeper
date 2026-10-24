<script setup lang="ts">
import { useId } from 'vue'

withDefaults(defineProps<{ size?: 'mark' | 'hero' }>(), { size: 'hero' })

const uid = useId()
const pq = uid + 'q'
const pc = uid + 'c'
</script>

<template>
  <svg class="prism" :class="size" viewBox="0 0 96 96" aria-hidden="true" focusable="false">
    <defs>
      <path :id="pq" d="M0-12 10.4-6V6L0 12-10.4 6V-6ZM0 0V12M0 0 10.4-6M0 0-10.4-6" />
      <g :id="pc">
        <use :href="'#' + pq" x="34" y="34" /><use :href="'#' + pq" x="56" y="30" />
        <use :href="'#' + pq" x="45" y="52" /><use :href="'#' + pq" x="66" y="52" />
        <use :href="'#' + pq" x="40" y="72" transform="rotate(8 40 72)" />
      </g>
    </defs>
    <g class="ch r" stroke="#ff2a2a"><use :href="'#' + pc" /></g>
    <g class="ch c" stroke="#2a7fff"><use :href="'#' + pc" /></g>
    <g class="ch g" stroke="#2aff2a"><use :href="'#' + pc" /></g>
    <g class="core" fill="#000" stroke="#fffdf9" stroke-width=".75"><use :href="'#' + pc" /></g>
  </svg>
</template>

<style scoped>
.prism {
  display: block;
  overflow: visible;
}
.mark {
  width: 32px;
  height: 32px;
}
.hero {
  width: var(--prism-size, 140px);
  height: auto;
}
.ch {
  fill: none;
  stroke-width: 2;
  mix-blend-mode: screen;
  animation: px var(--dur-prism) var(--ease) infinite alternate;
}
.r {
  transform: translate(-1.6px, 0.4px);
}
.c {
  transform: translate(1.6px, -0.4px);
  animation-name: pxc;
}
.g {
  transform: translate(0, 1.4px);
  animation-name: pxg;
}
.core {
  stroke-opacity: 0.85;
}
@keyframes px {
  to {
    transform: translate(-0.4px, 1.2px);
  }
}
@keyframes pxc {
  to {
    transform: translate(0.6px, -1.4px);
  }
}
@keyframes pxg {
  to {
    transform: translate(1.2px, 0.2px);
  }
}
@media (prefers-reduced-motion: reduce) {
  .ch {
    animation: none;
  }
}
</style>
