<script setup lang="ts">
const { isAdmin } = useRole()
const showNewPayment = ref(false)

// stopped payments stay in history but are hidden unless asked for
const showStopped = ref(false)
const demo = useDemo()
const stoppedCount = computed(() => demo.templates.value.filter(item => item.ended).length)
const shown = computed(() => demo.templates.value.filter(item => showStopped.value || !item.ended))
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

    <RecurringTable :templates="shown" :demo="demo.enabled.value" @stop="demo.stop" />

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
</style>
