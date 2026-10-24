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
    error.value = apiError(e)
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
        <p class="subtitle">Управление платежами вашей команды</p>

        <UiField label="Логин" class="inset">
          <UiInput v-model="form.login" autocomplete="username" maxlength="64" />
        </UiField>
        <UiField label="Пароль" class="inset">
          <UiInput v-model="form.password" type="password" autocomplete="current-password" />
        </UiField>

        <UiButton variant="primary" size="lg" block class="submit" :disabled="busy">Войти</UiButton>
        <p v-if="error" class="error-text">{{ error }}</p>
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
