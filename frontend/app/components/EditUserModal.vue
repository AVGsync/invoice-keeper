<script setup lang="ts">
import type { User, UserUpdateRequest } from '~/types/api'

const props = defineProps<{ user: User }>()
const emit = defineEmits<{ close: [], saved: [] }>()

const api = useApi()

const form = reactive({
  fullName: props.user.full_name ?? '',
  login: props.user.login ?? '',
})
const error = ref('')
const busy = ref(false)
// first click on "Удалить" only asks, second one deletes
const confirmDelete = ref(false)

async function run(action: () => Promise<unknown>) {
  error.value = ''
  busy.value = true
  try {
    await action()
    emit('saved')
  }
  catch (e) {
    error.value = apiError(e)
  }
  finally {
    busy.value = false
  }
}

function save() {
  // send only what changed
  const body: UserUpdateRequest = {}
  if (form.fullName !== props.user.full_name) body.full_name = form.fullName
  if (form.login !== props.user.login) body.login = form.login
  if (!body.full_name && !body.login) return emit('close')
  run(() => api.users.update(props.user.id!, body))
}

function remove() {
  if (!confirmDelete.value) {
    confirmDelete.value = true
    return
  }
  run(() => api.users.remove(props.user.id!))
}
</script>

<template>
  <UiModal title="Изменить пользователя" subtitle="Роль и пароль здесь не меняются." @close="$emit('close')">
    <UiField label="ФИО">
      <UiInput v-model="form.fullName" maxlength="64" />
    </UiField>

    <UiField label="Логин">
      <UiInput v-model="form.login" mono autocomplete="off" :filter="loginFilter" />
    </UiField>

    <p v-if="error" class="error-text">{{ error }}</p>

    <template #footer>
      <button type="button" class="delete" :disabled="busy" @click="remove">
        {{ confirmDelete ? 'Точно удалить?' : 'Удалить' }}
      </button>
      <UiButton size="md" class="cancel" @click="$emit('close')">Отмена</UiButton>
      <UiButton size="md" variant="primary" :disabled="busy" @click="save">Сохранить</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.delete {
  margin-right: auto;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--red);
}

.cancel {
  width: 110px;
}
</style>
