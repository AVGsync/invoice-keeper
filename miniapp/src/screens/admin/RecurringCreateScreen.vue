<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { recurring } from '../../stores/recurring'
import { users, usersState } from '../../stores/users'
import { parseMoneyInput } from '../../lib/money'
import { describeSchedule } from '../../lib/schedule'
import { today, addDays } from '../../lib/dates'
import { humanError } from '../../lib/humanError'
import { ApiError } from '../../api/errors'
import { haptic } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'
import { useDirtyGuard } from '../../composables/useDirtyGuard'
import Section from '../../components/Section.vue'
import Segmented from '../../components/Segmented.vue'
import type { Currency, Frequency } from '../../api/types'

const router = useRouter()

const description = ref('')
const amountInput = ref('')
const currency = ref<Currency>('RUB')
const startDate = ref(today())
const frequency = ref<Frequency>('monthly')
const interval = ref(1)
const assigneeId = ref('')
const search = ref('')
const submitting = ref(false)
const submitted = ref(false)
const formError = ref('')

const minDate = computed(() => addDays(today(), -365))
const maxDate = computed(() => addDays(today(), 3650))

onMounted(() => {
  if (!usersState.loaded) void users.load()
})

const amountMinor = computed(() => parseMoneyInput(amountInput.value))
const descTrim = computed(() => description.value.trim())

const filteredUsers = computed(() => {
  const list = usersState.items
  if (list.length <= 8 || !search.value.trim()) return list
  const q = search.value.trim().toLowerCase()
  return list.filter(
    (u) => u.full_name.toLowerCase().includes(q) || u.login.toLowerCase().includes(q),
  )
})
const showSearch = computed(() => usersState.items.length > 8)

const schedulePreview = computed(() =>
  describeSchedule(frequency.value, interval.value, startDate.value),
)

const valid = computed(
  () =>
    descTrim.value.length >= 1 &&
    descTrim.value.length <= 255 &&
    amountMinor.value !== null &&
    interval.value >= 1 &&
    interval.value <= 52 &&
    assigneeId.value !== '' &&
    startDate.value >= minDate.value,
)

const dirty = computed(
  () =>
    description.value !== '' ||
    amountInput.value !== '' ||
    assigneeId.value !== '' ||
    currency.value !== 'RUB' ||
    frequency.value !== 'monthly' ||
    interval.value !== 1,
)
useDirtyGuard(dirty, submitted)

async function submit(): Promise<void> {
  if (!valid.value || submitting.value) return
  formError.value = ''
  submitting.value = true
  try {
    await recurring.create({
      description: descTrim.value,
      amount: amountMinor.value!,
      currency: currency.value,
      frequency: frequency.value,
      interval: interval.value,
      start_date: startDate.value,
      assignee_id: assigneeId.value,
    })
    submitted.value = true
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    if (e instanceof ApiError) formError.value = humanError(e)
    else formError.value = 'Не удалось создать платёж'
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
    <h1 class="title">Новый платёж</h1>

    <label class="field">
      <span class="lbl">Название</span>
      <input v-model="description" type="text" maxlength="255" />
      <span class="counter hint">{{ descTrim.length }}/255</span>
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

    <label class="field">
      <span class="lbl">Ближайшая дата оплаты</span>
      <input v-model="startDate" type="date" :min="minDate" :max="maxDate" />
    </label>

    <div class="field">
      <span class="lbl">Периодичность</span>
      <Segmented
        :model-value="frequency"
        :options="[
          { value: 'monthly', label: 'Ежемесячно' },
          { value: 'weekly', label: 'Еженедельно' },
        ]"
        @update:model-value="(v) => (frequency = v)"
      />
      <div class="interval">
        <span>Каждые</span>
        <input v-model.number="interval" type="number" min="1" max="52" class="num int" />
        <span>{{ frequency === 'monthly' ? 'мес.' : 'нед.' }}</span>
      </div>
      <p class="preview hint">{{ schedulePreview }}</p>
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

    <p v-if="formError" class="form-error">{{ formError }}</p>
  </div>
</template>

<style scoped>
.field {
  display: block;
  margin-bottom: var(--sp-6);
}
.lbl {
  display: block;
  font-size: var(--fs-caption);
  font-weight: 500;
  color: var(--muted);
  margin: 0 0 8px var(--sp-1);
}
.lbl ~ .section {
  margin-top: var(--sp-2);
}
.counter {
  display: block;
  text-align: right;
  margin-top: 4px;
  font-size: var(--fs-micro);
  color: var(--fog);
  font-variant-numeric: tabular-nums;
}
.err {
  display: block;
  color: var(--bone);
  font-size: var(--fs-small);
  border-left: 2px solid rgba(var(--violet), 0.9);
  padding-left: 10px;
  margin-top: 8px;
}
.interval {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
}
.int {
  width: 72px;
  text-align: center;
}
.preview {
  color: var(--fog);
  font-size: var(--fs-small);
  margin: 8px 0 0;
}
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
.rlogin {
  color: var(--fog);
}
.form-error {
  display: block;
  color: var(--bone);
  font-size: var(--fs-small);
  border-left: 2px solid rgba(var(--violet), 0.9);
  padding-left: 10px;
  margin-top: 8px;
}
</style>
