<script setup lang="ts">
import { ref, computed, toRef, watch, onUnmounted } from 'vue'
import { session, link } from '../stores/session'
import { useMainButton } from '../composables/useMainButton'
import { haptic, setClosingConfirmation } from '../telegram/webapp'
import Horizon from '../components/Horizon.vue'

const login = ref('')
const password = ref('')
const showPassword = ref(false)

const valid = computed(() => login.value.trim() !== '' && password.value !== '')
const dirty = computed(() => login.value !== '')
const submitted = ref(false)

// LinkScreen рендерится вне RouterView, поэтому onBeforeRouteLeave здесь не работает (и шумит в dev).
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
    <Horizon size="hero" class="link-horizon" />
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
.link-horizon {
  --horizon-size: 88px;
  margin: var(--sp-8) 0 var(--sp-7);
}
.title {
  margin: 0 0 var(--sp-5);
}
.intro {
  color: var(--ash-text);
  font-size: var(--fs-body);
  max-width: 34ch;
  margin-bottom: var(--sp-8);
}
.field {
  display: block;
  margin-bottom: var(--sp-6);
}
.lbl {
  display: block;
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--muted);
  margin: 0 0 8px var(--sp-1);
}
.pw {
  position: relative;
}
.pw input {
  padding-right: 84px;
}
.toggle {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  border: 0;
  border-radius: var(--radius-pill);
  padding: 0 12px;
  background: transparent;
  color: var(--ash-text);
  font-size: var(--fs-caption);
  font-weight: 500;
  min-height: 44px;
}
.error {
  display: block;
  color: var(--bone);
  font-size: var(--fs-small);
  border-left: 2px solid var(--red);
  padding-left: 10px;
  margin-top: 8px;
}
</style>
