<script setup lang="ts">
const { login, home } = useAuth()

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
    const status = (e as { statusCode?: number }).statusCode
    if (status === 401) error.value = 'Неверный логин или пароль'
    // the server limits how often you can try to log in
    else if (status === 429) error.value = 'Слишком много попыток входа. Попробуйте позже.'
    else error.value = apiError(e)
  }
  finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login">
    <section class="left">
      <AppLogo />

      <form class="form" @submit.prevent="submit">
        <h1 class="title display">Вход <em class="italic">в ведомость</em></h1>
        <p class="subtitle">Регулярные платежи команды:<br>что и когда оплатить, что уже оплачено.</p>

        <UiField label="Логин">
          <UiInput v-model="form.login" autocomplete="username" maxlength="64" />
        </UiField>
        <UiField label="Пароль">
          <UiInput v-model="form.password" type="password" autocomplete="current-password" />
        </UiField>

        <UiButton variant="primary" size="lg" block class="submit" :disabled="busy">Войти</UiButton>
        <p v-if="error" class="error-text">{{ error }}</p>
        <p class="hint">Учётные записи выдаёт администратор.</p>
      </form>

      <p class="footer mono">v1.0 · Europe/Moscow</p>
    </section>

    <section class="side">
      <LoginReceipt />
    </section>
  </div>
</template>

<style scoped>
.login {
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 100vh;
}

.left {
  display: flex;
  flex-direction: column;
  padding: 40px 48px 38px;
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
  font-size: 64px;
  line-height: 84px;
  letter-spacing: -0.02em;
}

.title em {
  margin-left: 6px;
  font-size: 22px;
  letter-spacing: 0;
  color: var(--accent-ink);
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

.footer {
  margin-top: auto;
  font-size: 11px;
  color: var(--ink-3);
}

.side {
  display: flex;
  justify-content: center;
  padding-top: 196px;
  background: var(--muted-bg);
}
</style>
