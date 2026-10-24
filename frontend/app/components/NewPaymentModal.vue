<script setup lang="ts">
import type { Currency } from '~/utils/demo'

const emit = defineEmits<{ close: [] }>()

const demo = useDemo()

const form = reactive({
  title: '',
  amount: '',
  currency: 'RUB',
  due: demoToday,
  frequency: 'monthly',
})

const frequencies = [
  { value: 'monthly', label: 'Ежемесячно' },
  { value: 'yearly', label: 'Ежегодно' },
]

const currencies = [
  { value: 'RUB', label: 'RUB' },
  { value: 'USD', label: 'USD' },
  { value: 'EUR', label: 'EUR' },
]

// only a preview, the backend creates the real periods
const nextDates = computed(() => (form.due ? nextDueDates(form.due, 5, form.frequency === 'yearly') : []))

// the reminder goes out right away when the first due date is less than 3 days off
const remindsNow = computed(() => !!form.due && daysBetween(demoToday, form.due) < 3)

// demo mode only: the payment goes into the browser's demo data
const error = ref('')
function add() {
  const title = form.title.trim()
  const amount = parseMoney(form.amount)
  if (title.length < 2) error.value = 'Введите название'
  else if (amount <= 0) error.value = 'Введите сумму больше нуля'
  else if (!form.due) error.value = 'Выберите дату'
  else {
    demo.add({ title, amount, currency: form.currency as Currency, dueIso: form.due, yearly: form.frequency === 'yearly' })
    emit('close')
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
      <UiInput v-model="form.title" placeholder="Аренда склада" maxlength="64" />
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
        <UiDatePicker v-model="form.due" :today="demoToday" />
      </UiField>
      <UiField label="Периодичность">
        <UiDropdown v-model="form.frequency" :options="frequencies" placeholder="Периодичность" :clearable="false" />
      </UiField>
    </div>
    <p class="hint">31-е число: в коротких месяцах срок — последний день месяца.</p>

    <UiField label="Ответственный">
      <span class="who">
        <PersonCell :person="people.olga" />
        <span class="muted">{{ people.olga.persona }}</span>
        <UiIcon name="down" :size="14" class="muted caret" />
      </span>
    </UiField>

    <div class="preview">
      <p class="kicker">Следующие сроки</p>
      <p class="dates mono">
        <span v-for="(date, i) in nextDates" :key="date" :class="{ first: i === 0 }">{{ toRu(date) }}</span>
      </p>
    </div>

    <p v-if="remindsNow" class="notice">
      <span class="dot" />
      Срок ближе 3 дней — напоминание уйдёт в Telegram сразу, ответственной придёт упоминание.
    </p>

    <p v-if="error" class="error-text">{{ error }}</p>

    <template #footer>
      <UiButton size="md" class="cancel" @click="$emit('close')">Отмена</UiButton>
      <UiButton size="md" variant="primary" :soon="!demo.enabled.value" @click="add">Добавить платёж</UiButton>
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

.who {
  display: flex;
  align-items: center;
  gap: 20px;
  height: 40px;
  padding: 0 12px 0 10px;
  border: 1px solid var(--line-strong);
  border-radius: 6px;
  font-size: 13px;
}

.caret {
  margin-left: auto;
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
</style>
