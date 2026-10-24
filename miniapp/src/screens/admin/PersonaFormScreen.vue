<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { users, usersState } from '../../stores/users'
import { humanError } from '../../lib/humanError'
import { haptic, confirm as tgConfirm, alert as tgAlert } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'

const route = useRoute()
const router = useRouter()

const id = computed(() => (route.params.id as string | undefined) ?? null)
const name = ref('')
const description = ref('')
const submitting = ref(false)
const formError = ref('')
const loaded = ref(false)

onMounted(() => {
  if (!usersState.personasLoaded) void users.loadPersonas()
})

watch(
  () => usersState.personas,
  () => {
    if (loaded.value) return
    if (!id.value) {
      loaded.value = true
      return
    }
    const p = usersState.personas.find((x) => x.id === id.value)
    if (!p) return
    name.value = p.name
    description.value = p.description ?? ''
    loaded.value = true
  },
  { immediate: true, deep: true },
)

const current = computed(() => usersState.personas.find((x) => x.id === id.value))
const nameTrim = computed(() => name.value.trim())
const valid = computed(
  () =>
    nameTrim.value.length >= 1 &&
    nameTrim.value.length <= 64 &&
    description.value.length <= 255 &&
    (!current.value ||
      nameTrim.value !== current.value.name ||
      description.value.trim() !== (current.value.description ?? '')),
)

async function submit(): Promise<void> {
  if (!valid.value || submitting.value) return
  formError.value = ''
  submitting.value = true
  try {
    await users.savePersona(id.value, nameTrim.value, description.value.trim())
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    formError.value = humanError(e)
  } finally {
    submitting.value = false
  }
}

async function remove(): Promise<void> {
  if (!id.value || !current.value || submitting.value) return
  const n = current.value.users_count
  const text = n
    ? `Удалить персону «${current.value.name}»? У ${n} сотрудников она будет снята.`
    : `Удалить персону «${current.value.name}»?`
  if (!(await tgConfirm(text))) return
  submitting.value = true
  try {
    await users.removePersona(id.value)
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
  <div class="persona">
    <h1 class="title">{{ id ? 'Персона' : 'Новая персона' }}</h1>
    <label class="field">
      <span class="lbl">Название</span>
      <input v-model="name" type="text" maxlength="64" />
    </label>
    <label class="field">
      <span class="lbl">Описание (необязательно)</span>
      <textarea v-model="description" rows="3" maxlength="255" />
    </label>
    <p v-if="formError" class="form-error">{{ formError }}</p>
    <button v-if="id && current" class="btn btn-destructive remove" type="button" :disabled="submitting" @click="remove">
      Удалить персону
    </button>
  </div>
</template>

<style scoped>
.remove {
  margin-top: var(--sp-6);
}
</style>
