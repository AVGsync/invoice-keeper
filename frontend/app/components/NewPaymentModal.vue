<script setup lang="ts">
import type { NewPayment } from '~/composables/usePayments'
import type { Currency } from '~/utils/periods'

const emit = defineEmits<{ close: [] }>()

const payments = usePayments()

const form = reactive({
  title: '',
  amount: '',
  currency: 'RUB',
  due: payments.today.value,
  frequency: 'monthly',
  // user id of the person who pays
  assignee: '',
})

const frequencies = [
  { value: 'weekly', label: 'Еженедельно' },
  { value: 'monthly', label: 'Ежемесячно' },
  { value: 'quarterly', label: 'Ежеквартально' },
  { value: 'yearly', label: 'Ежегодно' },
]

const currencies = [
  { value: 'RUB', label: 'RUB' },
  { value: 'USD', label: 'USD' },
  { value: 'EUR', label: 'EUR' },
]

const assigneeOptions = computed(() => payments.assignees.value.map(({ id, person }) => ({
  value: id ?? person.name,
  label: person.name,
  meta: person.persona,
})))

// only a preview, the backend creates the real periods
const steps = { weekly: ['weekly', 1], monthly: ['monthly', 1], quarterly: ['monthly', 3], yearly: ['monthly', 12] } as const
const nextDates = computed(() => {
  if (!form.due) return []
  const [frequency, interval] = steps[form.frequency as NewPayment['frequency']]
  return dueDatesOf({ start_date: form.due, frequency, interval }, form.due, 5)
})

// the reminder goes out right away when the first due date is less than 3 days off
const remindsNow = computed(() => !!form.due && daysBetween(payments.today.value, form.due) < 3)

const error = ref('')
const busy = ref(false)

async function add() {
  const title = form.title.trim()
  const amount = parseMoney(form.amount)
  if (title.length < 2) error.value = 'Введите название'
  else if (amount <= 0) error.value = 'Введите сумму больше нуля'
  else if (!form.due) error.value = 'Выберите дату'
  else if (!form.assignee) error.value = 'Выберите ответственного'
  else {
    error.value = ''
    busy.value = true
    try {
      const picked = payments.assignees.value.find(item => (item.id ?? item.person.name) === form.assignee)
      await payments.add({
        title,
        amount,
        currency: form.currency as Currency,
        dueIso: form.due,
        frequency: form.frequency as NewPayment['frequency'],
        assigneeId: picked?.id,
      })
      emit('close')
    }
    catch (e) {
      error.value = apiError(e)
    }
    finally {
      busy.value = false
    }
  }
}
</script>

<template>
  <UiModal
    title="Новый регулярный платёж"
    subtitle="Периоды и напоминания создадутся автоматически."
    :width="540"
    @close="$emit('close')"
  >
    <UiField label="Название">
      <UiInput v-model="form.title" placeholder="Аренда склада" maxlength="255" />
    </UiField>

    <div class="pair">
      <UiField label="Сумма">
        <UiInput v-model="form.amount" mono placeholder="45 000,00" inputmode="decimal" :filter="moneyFilter" :format="moneyFormat" />
      </UiField>
      <UiField label="Валюта">
        <UiSegmented v-model="form.currency" :options="currencies" tone="accent" mono />
      </UiField>
    </div>

    <div class="pair even">
      <UiField label="Дата ближайшей оплаты">
        <UiDatePicker v-model="form.due" :today="payments.today.value" />
      </UiField>
      <UiField label="Периодичность">
        <UiDropdown v-model="form.frequency" :options="frequencies" placeholder="Периодичность" :clearable="false" />
      </UiField>
    </div>
    <p class="hint">31-е число: в коротких месяцах срок — последний день месяца.</p>

    <UiField label="Ответственный">
      <UiDropdown v-model="form.assignee" :options="assigneeOptions" placeholder="Кто платит" :clearable="false" />
    </UiField>

    <div class="preview">
      <p class="kicker">Следующие сроки</p>
      <p class="dates mono">
        <span v-for="(date, i) in nextDates" :key="date" :class="{ first: i === 0 }">{{ toRu(date) }}</span>
      </p>
    </div>

    <p v-if="remindsNow" class="notice">
      <span class="dot" />
      Срок ближе 3 дней — напоминание уйдёт в Telegram сразу, ответственному придёт упоминание.
    </p>

    <p v-if="error" class="error-text">{{ error }}</p>

    <template #footer>
      <UiButton size="md" class="cancel" @click="$emit('close')">Отмена</UiButton>
      <UiButton size="md" variant="primary" :disabled="busy" @click="add">Добавить платёж</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.pair {
  display: grid;
  grid-template-columns: 296px 1fr;
  gap: 16px;
}

.pair.even {
  grid-template-columns: 230px 1fr;
}

.hint {
  margin-top: -4px;
  font-size: 12px;
  color: var(--ink-3);
}



.preview {
  margin-top: 4px;
  padding: 14px 16px 16px;
  background: var(--subtle);
  border-radius: 8px;
}

.dates {
  display: flex;
  gap: 22px;
  margin-top: 13px;
  font-size: 12px;
  color: var(--ink-2);
}

.dates .first {
  color: var(--accent-ink);
  font-weight: 500;
}

.notice {
  display: flex;
  gap: 9px;
  margin-top: 8px;
  font-size: 13px;
  line-height: 19px;
  color: var(--ink-2);
}

.dot {
  flex-shrink: 0;
  width: 7px;
  height: 7px;
  margin-top: 6px;
  border-radius: 50%;
  background: var(--accent);
}

.cancel {
  width: 110px;
}

/* phones */
@media (max-width: 820px) {
  .pair,
  .pair.even {
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
  }

  .dates {
    flex-wrap: wrap;
    gap: 6px 16px;
  }

  .cancel {
    width: auto;
  }
}
</style>
