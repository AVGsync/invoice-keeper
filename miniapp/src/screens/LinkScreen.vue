<script setup lang="ts">
import { ref, computed, toRef, watch, onUnmounted } from 'vue'
import { session, link } from '../stores/session'
import { useMainButton } from '../composables/useMainButton'
import { haptic, setClosingConfirmation } from '../telegram/webapp'
import Prism from '../components/Prism.vue'

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
    <Prism size="hero" class="link-prism" />
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
.link-prism {
  --prism-size: 112px;
  margin: var(--sp-8) auto var(--sp-6);
}
.title {
  margin: 0 0 var(--sp-5);
}
.intro {
  color: var(--fog);
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
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--fog);
  margin-bottom: 4px;
}
.pw {
  position: relative;
}
.pw input {
  padding-right: 84px;
}
.toggle {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  border: 0;
  background: transparent;
  color: var(--bone);
  font-size: var(--fs-micro);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  min-height: 44px;
}
.error {
  display: block;
  color: var(--bone);
  font-size: var(--fs-small);
  border-left: 2px solid var(--bone);
  padding-left: 10px;
  margin-top: 8px;
}
</style>
