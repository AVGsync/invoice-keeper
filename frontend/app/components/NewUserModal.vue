<script setup lang="ts">
import type { Persona } from '~/types/api'

const props = defineProps<{ personas: Persona[] }>()
const emit = defineEmits<{ close: [], created: [], newPersona: [] }>()

const api = useApi()
const { generate } = usePasswordGenerator()

const form = reactive({
  fullName: '',
  login: '',
  password: generate(),
  persona: '',
})
const error = ref('')
const busy = ref(false)

const personaOptions = computed(() => props.personas.map(persona => ({
  value: persona.id ?? '',
  label: persona.name ?? '',
  meta: `${persona.users_count ?? 0} польз.`,
})))

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await api.users.create({
      full_name: form.fullName,
      login: form.login,
      password: form.password,
      ...(form.persona ? { persona_id: form.persona } : {}),
    })
    emit('created')
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
  <UiModal
    title="Новый пользователь"
    subtitle="Роль всегда «Пользователь». Администратор в системе один."
    @close="$emit('close')"
  >
    <UiField label="ФИО">
      <UiInput v-model="form.fullName" placeholder="Иван Петров" maxlength="64" />
    </UiField>

    <UiField label="Логин">
      <UiInput v-model="form.login" mono placeholder="ivan.petrov" autocomplete="off" :filter="loginFilter" />
    </UiField>

    <UiField label="Пароль">
      <template #aside>
        <button type="button" class="generate" @click="form.password = generate()">Сгенерировать</button>
      </template>
      <UiInput v-model="form.password" type="password" autocomplete="new-password" maxlength="72" />
    </UiField>

    <UiField label="Должность">
      <UiDropdown v-model="form.persona" :options="personaOptions" placeholder="Без должности">
        <template #footer>
          <button type="button" class="new-persona" @click="$emit('newPersona')">+&nbsp; Новая должность</button>
        </template>
      </UiDropdown>
    </UiField>

    <p v-if="error" class="error-text">{{ error }}</p>

    <template #footer>
      <UiButton size="md" class="cancel" @click="$emit('close')">Отмена</UiButton>
      <UiButton size="md" variant="primary" :disabled="busy" @click="submit">Создать пользователя</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.generate {
  padding: 0;
  background: none;
  border: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-2);
}

.new-persona {
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
}

.cancel {
  width: 110px;
}
</style>
