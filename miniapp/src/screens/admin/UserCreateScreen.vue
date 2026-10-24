<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { users, usersState } from '../../stores/users'
import { humanError } from '../../lib/humanError'
import { ApiError } from '../../api/errors'
import { haptic } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'
import { useDirtyGuard } from '../../composables/useDirtyGuard'

const router = useRouter()

const fullName = ref('')
const login = ref('')
const password = ref('')
const personaId = ref('')
const showPassword = ref(false)
const submitting = ref(false)
const submitted = ref(false)
const formError = ref('')

onMounted(() => {
  if (!usersState.personasLoaded) void users.loadPersonas()
})

const LOGIN_RE = /^[a-zA-Z0-9_.-]{3,32}$/
const passwordBytes = computed(() => new TextEncoder().encode(password.value).length)

const nameValid = computed(() => {
  const n = fullName.value.trim().length
  return n >= 3 && n <= 64
})
const loginValid = computed(() => LOGIN_RE.test(login.value))
const passwordValid = computed(() => password.value.length >= 8 && passwordBytes.value <= 72)

const valid = computed(() => nameValid.value && loginValid.value && passwordValid.value)
const dirty = computed(
  () => fullName.value !== '' || login.value !== '' || password.value !== '' || personaId.value !== '',
)
useDirtyGuard(dirty, submitted)

async function submit(): Promise<void> {
  if (!valid.value || submitting.value) return
  formError.value = ''
  submitting.value = true
  try {
    await users.create({
      full_name: fullName.value.trim(),
      login: login.value,
      password: password.value,
      persona_id: personaId.value || undefined,
    })
    submitted.value = true
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    if (e instanceof ApiError && e.status === 409) formError.value = 'Этот логин уже занят'
    else if (e instanceof ApiError) formError.value = humanError(e)
    else formError.value = 'Не удалось создать сотрудника'
  } finally {
    submitting.value = false
  }
}

useMainButton({
  text: 'Создать',
  enabled: valid,
  loading: submitting,
  onClick: submit,
})
</script>

<template>
  <div class="create">
    <h1 class="title">Новый сотрудник</h1>

    <label class="field">
      <span class="lbl">Имя</span>
      <input v-model="fullName" type="text" maxlength="64" autocapitalize="words" />
    </label>

    <label class="field">
      <span class="lbl">Логин</span>
      <input
        v-model="login"
        type="text"
        autocapitalize="off"
        autocorrect="off"
        spellcheck="false"
      />
      <span v-if="login && !loginValid" class="err">3–32 символа: латиница, цифры, . _ -</span>
    </label>

    <label class="field">
      <span class="lbl">Пароль</span>
      <div class="pw">
        <input v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" />
        <button type="button" class="toggle" @click="showPassword = !showPassword">
          {{ showPassword ? 'Скрыть' : 'Показать' }}
        </button>
      </div>
      <span v-if="password && !passwordValid" class="err">От 8 символов (не длиннее 72 байт)</span>
    </label>

    <label class="field">
      <span class="lbl">Персона</span>
      <select v-model="personaId">
        <option value="">Без персоны</option>
        <option v-for="p in usersState.personas" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
    </label>

    <p class="note hint">Новый сотрудник создаётся с ролью «Сотрудник»</p>
    <p v-if="formError" class="form-error">{{ formError }}</p>
  </div>
</template>

<style scoped>
.title {
  font-size: 22px;
  margin: 16px 0 16px;
}
.field {
  display: block;
  margin-bottom: 18px;
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
.err {
  color: var(--c-destructive);
  font-size: 13px;
}
.note {
  margin: 0 0 8px;
}
.form-error {
  color: var(--c-destructive);
}
</style>
