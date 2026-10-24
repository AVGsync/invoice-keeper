<script setup lang="ts">
import { computed } from 'vue'
import { session, retryBoot } from '../stores/session'
import { getWebApp } from '../telegram/webapp'

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
    <div class="emoji">
      <span v-if="phase === 'offline'">📡</span>
      <span v-else-if="phase === 'expired'">⏳</span>
      <span v-else>🚫</span>
    </div>
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
  padding: 48px 16px;
}
.emoji {
  font-size: 48px;
  margin-bottom: 16px;
}
.msg {
  font-size: 17px;
  margin-bottom: 16px;
}
.hint {
  color: var(--c-hint);
  font-size: 13px;
  margin-bottom: 16px;
}
</style>
