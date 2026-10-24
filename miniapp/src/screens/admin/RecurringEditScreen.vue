<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { recurring, recState } from '../../stores/recurring'
import { users, usersState } from '../../stores/users'
import { parseMoneyInput } from '../../lib/money'
import { humanError } from '../../lib/humanError'
import { haptic } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'
import Section from '../../components/Section.vue'
import Segmented from '../../components/Segmented.vue'
import Skeleton from '../../components/Skeleton.vue'
import type { Currency, RecurrentPayment } from '../../api/types'

const route = useRoute()
const router = useRouter()

const rec = ref<RecurrentPayment | null>(null)
const description = ref('')
const amountInput = ref('')
const currency = ref<Currency>('RUB')
const assigneeId = ref('')
const search = ref('')
const submitting = ref(false)
const formError = ref('')

onMounted(() => {
  if (!recState.loaded) void recurring.load()
  if (!usersState.loaded) void users.load()
})

watch(
  [() => route.params.id, () => recState.items],
  () => {
    if (rec.value) return
    const found = recState.items.find((r) => r.id === route.params.id)
    if (!found) return
    rec.value = found
    description.value = found.description
    amountInput.value = (found.amount / 100).toFixed(2).replace('.', ',')
    currency.value = found.currency
    assigneeId.value = found.assignee_id ?? ''
  },
  { immediate: true, deep: true },
)

const amountMinor = computed(() => parseMoneyInput(amountInput.value))
const descTrim = computed(() => description.value.trim())
const showSearch = computed(() => usersState.items.length > 8)
const filteredUsers = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!showSearch.value || !q) return usersState.items
  return usersState.items.filter((u) => u.full_name.toLowerCase().includes(q) || u.login.toLowerCase().includes(q))
})

const changed = computed(
  () =>
    !!rec.value &&
    (descTrim.value !== rec.value.description ||
      amountMinor.value !== rec.value.amount ||
      currency.value !== rec.value.currency ||
      (assigneeId.value !== '' && assigneeId.value !== (rec.value.assignee_id ?? ''))),
)
const valid = computed(
  () => descTrim.value.length >= 1 && descTrim.value.length <= 255 && amountMinor.value !== null && changed.value,
)

async function submit(): Promise<void> {
  if (!rec.value || !valid.value || submitting.value) return
  formError.value = ''
  submitting.value = true
  try {
    const body: Parameters<typeof recurring.update>[1] = {}
    if (descTrim.value !== rec.value.description) body.description = descTrim.value
    if (amountMinor.value !== rec.value.amount) body.amount = amountMinor.value!
    if (currency.value !== rec.value.currency) body.currency = currency.value
    if (assigneeId.value && assigneeId.value !== (rec.value.assignee_id ?? '')) body.assignee_id = assigneeId.value
    await recurring.update(rec.value.id, body)
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    formError.value = humanError(e)
  } finally {
    submitting.value = false
  }
}

useMainButton({ text: 'Сохранить', enabled: valid, loading: submitting, onClick: submit })
</script>

<template>
  <div class="edit">
    <h1 class="title">Изменить платёж</h1>
    <Skeleton v-if="!rec" :rows="4" />
    <template v-else>
      <label class="field">
        <span class="lbl">Название</span>
        <input v-model="description" type="text" maxlength="255" />
      </label>
      <label class="field">
        <span class="lbl">Сумма</span>
        <input v-model="amountInput" type="text" inputmode="decimal" class="num" placeholder="0,00" />
        <span v-if="amountInput && amountMinor === null" class="err">Введите сумму больше нуля</span>
      </label>
      <div class="field">
        <span class="lbl">Валюта</span>
        <Segmented
          :model-value="currency"
          :options="[
            { value: 'RUB', label: 'RUB' },
            { value: 'USD', label: 'USD' },
            { value: 'EUR', label: 'EUR' },
          ]"
          @update:model-value="(v) => (currency = v)"
        />
      </div>
      <div class="field">
        <span class="lbl">Ответственный</span>
        <input v-if="showSearch" v-model="search" type="text" placeholder="Поиск сотрудника" class="search" />
        <Section>
          <label v-for="u in filteredUsers" :key="u.id" class="radio-row row-active">
            <input v-model="assigneeId" type="radio" :value="u.id" name="assignee" />
            <span class="rname">{{ u.full_name }}</span>
            <span class="rlogin hint">@{{ u.login }}</span>
          </label>
        </Section>
      </div>
      <p class="hint">График (даты и периодичность) здесь не меняется: остановите платёж и создайте новый.</p>
    </template>
    <p v-if="formError" class="form-error">{{ formError }}</p>
  </div>
</template>

<style scoped>
.search {
  margin-bottom: var(--sp-2);
}
.radio-row {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.radio-row input[type='radio'] {
  appearance: none;
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
  min-height: 0;
  margin: 0;
  padding: 3px;
  border: 1px solid var(--fog);
  border-radius: 50%;
  background-clip: content-box;
}
.radio-row input[type='radio']:checked {
  border-color: var(--bone);
  background-color: var(--bone);
}
.rname {
  flex: 1;
  font-size: var(--fs-body-l);
}
</style>
