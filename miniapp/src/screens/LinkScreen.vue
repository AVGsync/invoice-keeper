<script setup lang="ts">
import { ref, computed, toRef, watch, onUnmounted } from 'vue'
import { session, link } from '../stores/session'
import { useMainButton } from '../composables/useMainButton'
import { haptic, setClosingConfirmation } from '../telegram/webapp'

const login = ref('')
const password = ref('')
const showPassword = ref(false)

const valid = computed(() => login.value.trim() !== '' && password.value !== '')
const dirty = computed(() => login.value !== '')
const submitted = ref(false)

// LinkScreen рендерится вне RouterView, поэтому onBeforeRouteLeave здесь не работает
// (и шумит в dev). Ограничиваемся подтверждением закрытия Telegram.
watch(dirty, (v) => setClosingConfirmation(v && !submitted.value), { immediate: true })
onUnmounted(() => setClosingConfirmation(false))

async function submit(): Promise<void> {
  if (!valid.value || session.linkLoading) return
  await link(login.value.trim(), password.value)
  if (session.phase === 'ready') {
    submitted.value = true
    haptic.success()
    password.value = ''
  }
}

useMainButton({
  text: 'Войти',
  enabled: valid,
  loading: toRef(session, 'linkLoading'),
  onClick: submit,
})
</script>

<template>
  <div class="link">
    <h1 class="title">Привязка аккаунта</h1>
    <p class="intro">
      Войдите один раз логином и паролем от Invoice Keeper — дальше вход будет по Telegram
    </p>

    <label class="field">
      <span class="lbl">Логин</span>
      <input
        v-model="login"
        type="text"
        autocomplete="username"
        autocapitalize="off"
        autocorrect="off"
        spellcheck="false"
        inputmode="text"
      />
    </label>

    <label class="field">
      <span class="lbl">Пароль</span>
      <div class="pw">
        <input
          v-model="password"
          :type="showPassword ? 'text' : 'password'"
          autocomplete="current-password"
          @keyup.enter="submit"
        />
        <button type="button" class="toggle" @click="showPassword = !showPassword">
          {{ showPassword ? 'Скрыть' : 'Показать' }}
        </button>
      </div>
    </label>

    <p v-if="session.linkError" class="error">{{ session.linkError }}</p>
  </div>
</template>

<style scoped>
.link {
  padding-top: 24px;
}
.title {
  font-size: 22px;
  margin-bottom: 8px;
}
.intro {
  color: var(--c-hint);
  margin-bottom: 20px;
}
.field {
  display: block;
  margin-bottom: 16px;
}
.lbl {
  display: block;
  font-size: 13px;
  color: var(--c-hint);
  margin-bottom: 6px;
}
.pw {
  position: relative;
}
.toggle {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  border: 0;
  background: transparent;
  color: var(--c-link);
  font-size: 14px;
}
.error {
  color: var(--c-destructive);
  margin-top: 8px;
}
</style>
