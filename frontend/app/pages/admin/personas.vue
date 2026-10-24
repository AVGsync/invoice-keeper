<script setup lang="ts">
import type { Persona } from '~/types/api'

definePageMeta({ layout: 'panel' })

const api = useApi()

const personas = ref<Persona[]>([])
const loadError = ref('')
// true until the first answer arrives
const loading = ref(true)

const open = ref(false)
// persona being edited, empty = creating a new one
const editing = ref<Persona>()

async function load() {
  try {
    personas.value = (await api.personas.list()).items ?? []
    loadError.value = ''
  }
  catch (e) {
    loadError.value = apiError(e)
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

function openModal(persona?: Persona) {
  editing.value = persona
  open.value = true
}

function saved() {
  open.value = false
  load()
}
</script>

<template>
  <div>
    <AppPageHeader title="Персоны" subtitle="Должности в команде. На права не влияют, их задаёт роль">
      <template #actions>
        <UiButton variant="primary" class="new" @click="openModal()">Новая персона</UiButton>
      </template>
    </AppPageHeader>

    <UiState v-if="loadError" kind="error" class="card load-error" @retry="load">Не удалось загрузить: {{ loadError }}</UiState>

    <PersonasTable :personas="personas" :loading="loading" @edit="openModal" />

    <Transition name="modal">
      <PersonaModal v-if="open" :persona="editing" @close="open = false" @saved="saved" />
    </Transition>
  </div>
</template>

<style scoped>
.load-error {
  margin-bottom: 16px;
}

.new {
  width: 160px;
}
</style>
