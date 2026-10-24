<script setup lang="ts">
import type { Template } from '~/utils/demo'

const { isAdmin } = useRole()
const showNewPayment = ref(false)

// stopped payments stay in history but are hidden unless asked for
const showStopped = ref(false)
const payments = usePayments()
onMounted(payments.load)
const stoppedCount = computed(() => payments.templates.value.filter(item => item.ended).length)
const shown = computed(() => payments.templates.value.filter(item => showStopped.value || !item.ended))

function stop(template: Template) {
  payments.stop(template).catch(() => {})
}
</script>

<template>
  <div>
    <AppPageHeader
      title="Регулярные платежи"
      :subtitle="isAdmin
        ? 'Шаблоны: из них автоматически создаются периоды и напоминания'
        : 'Что, сколько и кто платит — периоды создаются автоматически'"
    >
      <template v-if="isAdmin" #actions>
        <UiButton variant="primary" class="new" @click="showNewPayment = true">Новый платёж</UiButton>
      </template>
    </AppPageHeader>

    <div class="tools">
      <UiSwitch v-model="showStopped">Показать прекращённые <span class="mono muted">{{ stoppedCount }}</span></UiSwitch>
    </div>

    <UiState v-if="payments.pending.value" class="state">Загружаем платежи…</UiState>
    <UiState v-else-if="payments.loadError.value" kind="error" class="state" @retry="payments.load">
      Не удалось загрузить: {{ payments.loadError.value }}
    </UiState>
    <UiState v-else-if="!shown.length" class="state">
      {{ isAdmin ? 'Регулярных платежей пока нет. Добавьте первый — «Новый платёж».' : 'Регулярных платежей пока нет.' }}
    </UiState>
    <RecurringTable v-else :templates="shown" @stop="stop" />

    <p v-if="!isAdmin" class="read-only">
      <span class="dot" />
      Только просмотр: добавлять и прекращать регулярные платежи может администратор.
    </p>

    <Transition name="modal">
      <NewPaymentModal v-if="showNewPayment" @close="showNewPayment = false" />
    </Transition>
  </div>
</template>

<style scoped>
.tools {
  display: flex;
  justify-content: flex-end;
  margin: -6px 0 12px;
}

.read-only {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 22px;
  font-size: 13px;
  color: var(--ink-3);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ink-3);
}

.new {
  width: 160px;
}

.state {
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
}
</style>
