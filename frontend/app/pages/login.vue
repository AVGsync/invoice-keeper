<script setup lang="ts">
const { login, afterTelegramLogin, home } = useAuth()
const api = useApi()

const form = reactive({ login: '', password: '' })
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await login({ ...form })
    await navigateTo(home.value)
  }
  catch (e) {
    error.value = apiError(e)
  }
  finally {
    busy.value = false
  }
}

// Telegram sign-in: open the bot, confirm with the button there, meanwhile poll every 2 seconds
const tg = reactive({ waiting: false, link: '' })
let tgTimer: ReturnType<typeof setTimeout> | undefined

function tgStop() {
  clearTimeout(tgTimer)
  tg.waiting = false
  tg.link = ''
}

async function tgPoll(code: string, until: number) {
  try {
    const res = await api.auth.telegramPoll(code)
    if (res.status === 'ok') {
      tgStop()
      await afterTelegramLogin()
      await navigateTo(home.value)
      return
    }
  }
  catch (e) {
    tgStop()
    const status = (e as { statusCode?: number }).statusCode
    error.value = status === 410 ? 'Вход отменён или время вышло. Попробуйте ещё раз.' : apiError(e)
    return
  }
  if (Date.now() > until) {
    tgStop()
    error.value = 'Время вышло. Попробуйте ещё раз.'
    return
  }
  tgTimer = setTimeout(() => tgPoll(code, until), 2000)
}

async function tgLogin() {
  error.value = ''
  tgStop()
  try {
    const res = await api.auth.telegramStart()
    tg.link = res.deep_link
    tg.waiting = true
    window.open(res.deep_link, '_blank', 'noopener')
    tgTimer = setTimeout(() => tgPoll(res.code, Date.now() + res.expires_in * 1000), 2000)
  }
  catch (e) {
    const status = (e as { statusCode?: number }).statusCode
    error.value = status === 503 ? 'Вход через Telegram сейчас недоступен.' : apiError(e)
  }
}

onBeforeUnmount(() => clearTimeout(tgTimer))
</script>

<template>
  <div class="login">
    <section class="left">
      <AppLogo />
      <AppMascot class="mascot" pose="wave" :size="150" />

      <form class="form" @submit.prevent="submit">
        <h1 class="title display">Вход <em class="italic">в ведомость</em></h1>
        <p class="subtitle">Управление платежами вашей команды</p>

        <UiField label="Логин" class="inset">
          <UiInput v-model="form.login" autocomplete="username" maxlength="64" />
        </UiField>
        <UiField label="Пароль" class="inset">
          <UiInput v-model="form.password" type="password" autocomplete="current-password" />
        </UiField>

        <UiButton variant="primary" size="lg" block class="submit" :disabled="busy">Войти</UiButton>
        <p v-if="error" class="error-text">{{ error }}</p>

        <div class="or"><span>или</span></div>
        <UiButton v-if="!tg.waiting" type="button" size="lg" block @click="tgLogin">Войти через Telegram</UiButton>
        <div v-else class="tg-wait">
          <p class="hint">Подтвердите вход в боте. Если Telegram не открылся, <a :href="tg.link" target="_blank" rel="noopener">откройте бота</a>.</p>
          <UiButton type="button" size="sm" @click="tgStop">Отмена</UiButton>
        </div>
      </form>

      <p class="footer mono">Europe/Moscow</p>
    </section>

    <section class="side">
      <LoginBackdrop />
      <LoginReceipt class="over-rings" />
    </section>
  </div>
</template>

<style scoped>
/* «Логин» and «Пароль» start where the field's round end is over, in line with the text inside */
.inset :deep(.top) {
  padding-left: 16px;
}

.login {
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 100vh;
}

.left {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 40px 48px 38px;
}

/* in the free space right of the form, hidden when it gets tight */
.mascot {
  position: absolute;
  top: 190px;
  left: 580px;
}

@media (max-width: 1360px) {
  .mascot {
    display: none;
  }
}

.form {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 340px;
  margin: 158px 0 0 152px;
}

/* the title is allowed to be wider than the form below it */
.title {
  margin-left: -2px;
  white-space: nowrap;
  font-size: 44px;
  line-height: 56px;
  letter-spacing: -0.03em;
}

.title em {
  margin-left: 6px;
  font-size: 20px;
  font-weight: 400;
  letter-spacing: 0;
  color: var(--ink-3);
}

.subtitle {
  margin-bottom: 20px;
  font-size: 15px;
  line-height: 22px;
  color: var(--ink-2);
}

.submit {
  margin-top: 12px;
}

.hint {
  margin-top: 6px;
  font-size: 13px;
  color: var(--ink-3);
}

.or {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 8px 0 2px;
  font-size: 12px;
  color: var(--ink-3);
}

.or::before,
.or::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--line);
}

.tg-wait {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
}

.tg-wait a {
  color: var(--ink-2);
  text-decoration: underline;
}

.footer {
  margin-top: auto;
  font-size: 12px;
  color: var(--ink-3);
}

.side {
  /* the rings behind the receipt go past the edges, the section cuts them off */
  position: relative;
  overflow: hidden;
  display: flex;
  justify-content: center;
  padding-top: 196px;
  background: var(--subtle);
  border-left: 1px solid var(--line);
}

/* above the rings */
.over-rings {
  position: relative;
}
</style>
