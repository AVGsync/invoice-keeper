<script setup lang="ts">
import type { Period } from '~/utils/demo'

const props = defineProps<{ period: Period }>()
const emit = defineEmits<{ close: [] }>()

const demo = useDemo()

// the table row this popover belongs to counts as inside, so clicking that row again toggles it
const root = ref<HTMLElement>()
useClickOutside(() => root.value?.parentElement, () => emit('close'))

// an already paid period opens with its date, to fix it or take the mark off
const alreadyPaid = !!props.period.paidAt
const paidAt = ref(alreadyPaid ? fromRu(props.period.paidAt!) : demoToday)
// paid after the due date counts as late
const late = computed(() => !!paidAt.value && paidAt.value > fromRu(props.period.due))

function confirm() {
  demo.markPaid(props.period, paidAt.value)
  emit('close')
}

function unmark() {
  demo.unmark(props.period)
  emit('close')
}
</script>

<template>
  <div ref="root" class="card popover" @click.stop>
    <h3 class="title display">{{ alreadyPaid ? 'Оплата' : 'Отметить оплату' }}</h3>
    <p class="meta">{{ period.title }} · {{ period.amount }} · срок {{ period.due }}</p>

    <UiField label="Фактическая дата оплаты">
      <UiDatePicker v-model="paidAt" :today="demoToday" />
    </UiField>
    <p v-if="late" class="hint late">Позже срока — будет «оплачено с опозданием».</p>
    <p v-else class="hint">Оплата в день срока и раньше — вовремя.</p>

    <div class="buttons">
      <UiButton v-if="alreadyPaid" size="md" :soon="!demo.enabled.value" @click="unmark">Снять отметку</UiButton>
      <UiButton v-else size="md" @click="$emit('close')">Отмена</UiButton>
      <UiButton size="md" variant="primary" :soon="!demo.enabled.value" @click="confirm">
        {{ alreadyPaid ? 'Сохранить' : 'Подтвердить' }}
      </UiButton>
    </div>
  </div>
</template>

<style scoped>
.popover {
  width: 296px;
  padding: 10px 18px 18px;
}

.title {
  font-size: 22px;
}

.meta {
  margin: 2px 0 8px;
  font-size: 12px;
  color: var(--ink-3);
}

.hint {
  margin: 8px 0 12px;
  font-size: 12px;
  color: var(--ink-2);
}

.hint.late {
  color: var(--amber);
}

.buttons {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 8px;
}
</style>
