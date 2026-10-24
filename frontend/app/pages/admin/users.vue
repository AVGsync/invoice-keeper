<script setup lang="ts">
import type { Persona, User } from '~/types/api'

definePageMeta({ layout: 'panel' })

const api = useApi()

const users = ref<User[]>([])
const personas = ref<Persona[]>([])
const loadError = ref('')
// true until the first answer arrives
const loading = ref(true)

// which user modal is open. the persona modal can open on top of the user form
const modal = ref<'newUser' | 'editUser'>()
const personaOpen = ref(false)
const editing = ref<User>()
// persona being edited in the panel, empty = creating a new one
const editingPersona = ref<Persona>()

async function load() {
  try {
    const [userList, personaList] = await Promise.all([api.users.list(), api.personas.list()])
    users.value = userList.items ?? []
    personas.value = personaList.items ?? []
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

function edit(user: User) {
  editing.value = user
  modal.value = 'editUser'
}

function done() {
  modal.value = undefined
  load()
}

function openPersona(persona?: Persona) {
  editingPersona.value = persona
  personaOpen.value = true
}

async function personaSaved() {
  personaOpen.value = false
  await load()
}
</script>

<template>
  <div>
    <AppPageHeader title="Пользователи">
      <template #actions>
        <UiButton variant="primary" class="new" @click="modal = 'newUser'">Новый пользователь</UiButton>
      </template>
    </AppPageHeader>

    <UiState v-if="loadError" kind="error" class="card load-error" @retry="load">Не удалось загрузить: {{ loadError }}</UiState>

    <div class="layout">
      <UsersTable :users="users" :loading="loading" @edit="edit" />
      <PersonasPanel :items="personas" :loading="loading" @add="openPersona()" @edit="openPersona" />
    </div>

    <Transition name="modal">
      <NewUserModal
        v-if="modal === 'newUser'"
        :personas="personas"
        @close="modal = undefined"
        @created="done"
        @new-persona="openPersona()"
      />
    </Transition>
    <Transition name="modal">
      <EditUserModal v-if="modal === 'editUser' && editing" :user="editing" @close="modal = undefined" @saved="done" />
    </Transition>
    <Transition name="modal">
      <PersonaModal v-if="personaOpen" :persona="editingPersona" @close="personaOpen = false" @saved="personaSaved" />
    </Transition>
  </div>
</template>

<style scoped>
/* when the window is narrow the positions card goes under the table, so the table keeps its full width */
.layout {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 16px;
}

.layout > :first-child {
  flex: 1 1 790px;
  min-width: 0;
}

.load-error {
  margin-bottom: 16px;
}

.new {
  width: 160px;
}
</style>
