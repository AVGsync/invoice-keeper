<script setup lang="ts">
import type { Persona, UpdatePersonaRequest } from '~/types/api'

// without `persona` it creates a new one, with it — edits that persona
const props = defineProps<{ persona?: Persona }>()
const emit = defineEmits<{ close: [], saved: [] }>()

const api = useApi()

const form = reactive({
  name: props.persona?.name ?? '',
  description: props.persona?.description ?? '',
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
  if (!props.persona) {
    return run(() => api.personas.create({
      name: form.name,
      ...(form.description ? { description: form.description } : {}),
    }))
  }

  // send only what changed, an empty description removes it
  const body: UpdatePersonaRequest = {}
  if (form.name !== props.persona.name) body.name = form.name
  if (form.description !== (props.persona.description ?? '')) body.description = form.description
  if (!Object.keys(body).length) return emit('close')
  run(() => api.personas.update(props.persona!.id!, body))
}

function remove() {
  if (!confirmDelete.value) {
    confirmDelete.value = true
    return
  }
  run(() => api.personas.remove(props.persona!.id!))
}
</script>

<template>
  <UiModal
    :title="persona ? 'Изменить персону' : 'Новая персона'"
    subtitle="Должность в команде, на права не влияет."
    @close="$emit('close')"
  >
    <UiField label="Название">
      <UiInput v-model="form.name" placeholder="Бухгалтер" maxlength="64" />
    </UiField>

    <UiField label="Описание">
      <UiInput v-model="form.description" placeholder="Оплачивает аренду и подписки" maxlength="255" />
    </UiField>

    <p v-if="error" class="error-text">{{ error }}</p>

    <template #footer>
      <button v-if="persona" type="button" class="delete" :disabled="busy" @click="remove">
        {{ confirmDelete ? 'Точно удалить?' : 'Удалить' }}
      </button>
      <UiButton size="md" class="cancel" @click="$emit('close')">Отмена</UiButton>
      <UiButton size="md" variant="primary" :disabled="busy" @click="save">
        {{ persona ? 'Сохранить' : 'Создать персону' }}
      </UiButton>
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
