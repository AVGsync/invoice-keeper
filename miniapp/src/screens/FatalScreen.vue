<script setup lang="ts">
import { computed } from 'vue'
import { session, retryBoot } from '../stores/session'
import { getWebApp } from '../telegram/webapp'
import Prism from '../components/Prism.vue'

const phase = computed(() => session.phase)
// Закрыть — для терминальных фаз, где повтор не поможет.
const showClose = computed(() => phase.value === 'expired' || phase.value === 'invalid')
const showRetry = computed(() => phase.value === 'rate_limited' || phase.value === 'offline')
const isDev = import.meta.env.DEV
const showDevHint = computed(() => phase.value === 'no_telegram' && isDev)

function close(): void {
  getWebApp()?.close()
}
</script>

<template>
  <div class="fatal">
    <Prism size="hero" class="fatal-prism" />
    <p class="msg">{{ session.fatalReason }}</p>
    <p v-if="showDevHint" class="hint">
      Dev: задайте токен бота в miniapp/.env.development.local
    </p>
    <button v-if="showClose" class="btn" type="button" @click="close">Закрыть</button>
    <button v-if="showRetry" class="btn" type="button" @click="retryBoot">Повторить</button>
  </div>
</template>

<style scoped>
.fatal {
  text-align: center;
  padding: var(--sp-9) 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--sp-5);
}
.fatal-prism {
  --prism-size: 120px;
}
.msg {
  font-size: var(--fs-title);
  line-height: 1.25;
  letter-spacing: -0.01em;
  max-width: 28ch;
  margin: 0;
}
.hint {
  font-size: var(--fs-small);
}
.fatal .btn {
  min-width: 200px;
}
</style>
