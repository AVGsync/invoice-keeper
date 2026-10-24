<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { users, usersState } from '../../stores/users'
import { humanError } from '../../lib/humanError'
import { ApiError } from '../../api/errors'
import { haptic, confirm as tgConfirm, alert as tgAlert } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'
import Skeleton from '../../components/Skeleton.vue'
import type { User } from '../../api/types'

const route = useRoute()
const router = useRouter()

const user = ref<User | null>(null)
const fullName = ref('')
const login = ref('')
const personaId = ref('')
const password = ref('')
const showPassword = ref(false)
const submitting = ref(false)
const formError = ref('')

const LOGIN_RE = /^[a-zA-Z0-9_.-]{3,32}$/

onMounted(() => {
  if (!usersState.loaded) void users.load()
  if (!usersState.personasLoaded) void users.loadPersonas()
})

watch(
  [() => route.params.id, () => usersState.items],
  () => {
    if (user.value) return
    const found = usersState.items.find((u) => u.id === route.params.id)
    if (!found) return
    user.value = found
    fullName.value = found.full_name
    login.value = found.login
    personaId.value = found.persona?.id ?? ''
  },
  { immediate: true, deep: true },
)

const nameValid = computed(() => fullName.value.trim().length >= 3 && fullName.value.trim().length <= 64)
const loginValid = computed(() => LOGIN_RE.test(login.value))
const passwordBytes = computed(() => new TextEncoder().encode(password.value).length)
const passwordValid = computed(() => password.value === '' || (password.value.length >= 8 && passwordBytes.value <= 72))

const changed = computed(
  () =>
    !!user.value &&
    (fullName.value.trim() !== user.value.full_name ||
      login.value !== user.value.login ||
      personaId.value !== (user.value.persona?.id ?? '') ||
      password.value !== ''),
)
const valid = computed(() => nameValid.value && loginValid.value && passwordValid.value && changed.value)

async function submit(): Promise<void> {
  if (!user.value || !valid.value || submitting.value) return
  formError.value = ''
  submitting.value = true
  try {
    const body: Parameters<typeof users.update>[1] = {}
    if (fullName.value.trim() !== user.value.full_name) body.full_name = fullName.value.trim()
    if (login.value !== user.value.login) body.login = login.value
    if (personaId.value !== (user.value.persona?.id ?? '')) body.persona_id = personaId.value
    if (password.value) body.password = password.value
    await users.update(user.value.id, body)
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    formError.value = e instanceof ApiError && e.status === 409 ? 'Этот логин уже занят' : humanError(e)
  } finally {
    submitting.value = false
  }
}

async function remove(): Promise<void> {
  if (!user.value || submitting.value) return
  if (!(await tgConfirm(`Удалить сотрудника «${user.value.full_name}»?`))) return
  submitting.value = true
  try {
    await users.remove(user.value.id)
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    await tgAlert(humanError(e))
  } finally {
    submitting.value = false
  }
}

useMainButton({ text: 'Сохранить', enabled: valid, loading: submitting, onClick: submit })
</script>

<template>
  <div class="edit">
    <h1 class="title">Сотрудник</h1>
    <Skeleton v-if="!user" :rows="4" />
    <template v-else>
      <label class="field">
        <span class="lbl">Имя</span>
        <input v-model="fullName" type="text" maxlength="64" autocapitalize="words" />
      </label>
      <label class="field">
        <span class="lbl">Логин</span>
        <input v-model="login" type="text" autocapitalize="off" autocorrect="off" spellcheck="false" />
        <span v-if="login && !loginValid" class="err">3–32 символа: латиница, цифры, . _ -</span>
      </label>
      <label class="field">
        <span class="lbl">Персона</span>
        <select v-model="personaId">
          <option value="">Без персоны</option>
          <option v-for="p in usersState.personas" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </label>
      <label class="field">
        <span class="lbl">Новый пароль (необязательно)</span>
        <div class="pw">
          <input v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" />
          <button type="button" class="toggle" @click="showPassword = !showPassword">
            {{ showPassword ? 'Скрыть' : 'Показать' }}
          </button>
        </div>
        <span v-if="password && !passwordValid" class="err">От 8 символов (не длиннее 72 байт)</span>
      </label>

      <p v-if="formError" class="form-error">{{ formError }}</p>

      <button class="btn btn-destructive remove" type="button" :disabled="submitting" @click="remove">
        Удалить сотрудника
      </button>
    </template>
  </div>
</template>

<style scoped>
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
.remove {
  margin-top: var(--sp-6);
}
</style>
