<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { recurring, recState } from '../../stores/recurring'
import { occState, occurrences } from '../../stores/occurrences'
import { describeSchedule } from '../../lib/schedule'
import { formatDate, today } from '../../lib/dates'
import { humanError } from '../../lib/humanError'
import { haptic, confirm as tgConfirm, alert as tgAlert } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'
import Segmented from '../../components/Segmented.vue'
import Section from '../../components/Section.vue'
import Money from '../../components/Money.vue'
import Skeleton from '../../components/Skeleton.vue'
import ErrorState from '../../components/ErrorState.vue'
import EmptyState from '../../components/EmptyState.vue'

const route = useRoute()
const router = useRouter()
type Tab = 'active' | 'stopped'
const tab = ref<Tab>('active')
const busyId = ref<string | null>(null)

// Подсветка конкретного шаблона (переход из ответа ассистента: ?focus=<id>).
const focusId = computed(() => (route.query.focus as string | undefined) || null)

onMounted(() => {
  if (!recState.loaded) void recurring.load()
  // Нужны периоды, чтобы показать ближайшую дату.
  void occurrences.load()
})

watch(
  [() => recState.loaded, focusId],
  async ([loaded, id]) => {
    if (!loaded || !id) return
    const r = recState.items.find((x) => x.id === id)
    if (r) tab.value = r.stopped_at ? 'stopped' : 'active'
    await nextTick()
    document.getElementById('rec-' + id)?.scrollIntoView({ block: 'center' })
  },
  { immediate: true },
)

const items = computed(() =>
  recState.items.filter((r) => (tab.value === 'active' ? !r.stopped_at : !!r.stopped_at)),
)
const firstLoad = computed(() => recState.loading && !recState.loaded)

// Ближайшая дата: минимальная due_date незакрытого периода этого шаблона.
function nextDue(recId: string): string | null {
  let min: string | null = null
  for (const o of occState.items) {
    if (o.recurrent_payment_id !== recId) continue
    if (o.status !== 'pending' && o.status !== 'overdue') continue
    if (min === null || o.due_date < min) min = o.due_date
  }
  return min
}
const todayStr = computed(() => today())

async function stop(id: string, description: string): Promise<void> {
  const ok = await tgConfirm(
    `Остановить «${description}»? Новые периоды не будут создаваться, история сохранится.`,
  )
  if (!ok) return
  busyId.value = id
  try {
    await recurring.stop(id)
    haptic.success()
  } catch (e) {
    haptic.error()
    await tgAlert(humanError(e))
  } finally {
    busyId.value = null
  }
}

useMainButton({
  text: 'Новый платёж',
  onClick: () => {
    void router.push('/admin/recurring/new')
  },
})
</script>

<template>
  <div class="recurring">
    <h1 class="title">Регулярные платежи</h1>
    <Segmented
      :model-value="tab"
      :options="[
        { value: 'active', label: 'Активные' },
        { value: 'stopped', label: 'Остановленные' },
      ]"
      @update:model-value="(v) => (tab = v)"
    />

    <Skeleton v-if="firstLoad" :rows="5" />
    <ErrorState v-else-if="recState.error && !recState.loaded" :error="recState.error" @retry="recurring.load()" />
    <EmptyState v-else-if="!items.length" text="Регулярных платежей пока нет" />
    <Section v-else>
      <div
        v-for="r in items"
        :id="'rec-' + r.id"
        :key="r.id"
        class="rrow"
        :class="{ focus: r.id === focusId }"
      >
        <div class="top">
          <span class="desc">{{ r.description }}</span>
          <Money :amount="r.amount" :currency="r.currency" />
        </div>
        <div class="sub hint">{{ describeSchedule(r.frequency, r.interval, r.start_date) }}</div>
        <div class="sub hint">Ответственный: {{ r.assignee_name || '—' }}</div>
        <div v-if="!r.stopped_at && nextDue(r.id)" class="sub hint">
          Ближайшая: {{ formatDate(nextDue(r.id)!, { today: todayStr }) }}
        </div>
        <div v-else-if="!r.stopped_at" class="sub hint">
          Начало графика: {{ formatDate(r.start_date, { today: todayStr }) }}
        </div>
        <button
          v-if="!r.stopped_at"
          class="btn btn-secondary edit"
          type="button"
          @click="router.push('/admin/recurring/' + r.id + '/edit')"
        >
          Изменить
        </button>
        <button
          v-if="!r.stopped_at"
          class="btn btn-destructive stop"
          type="button"
          :disabled="busyId === r.id"
          @click="stop(r.id, r.description)"
        >
          Остановить
        </button>
      </div>
    </Section>
  </div>
</template>

<style scoped>
.rrow {
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.rrow.focus {
  outline: 1px solid var(--accent);
  outline-offset: -1px;
  border-radius: 12px;
  padding-left: var(--sp-3);
  padding-right: var(--sp-3);
}
.top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 4px;
}
.desc {
  font-size: var(--fs-body-l);
  font-weight: 400;
}
.sub {
  font-size: var(--fs-small);
  color: var(--fog);
  margin: 1px 0;
}
.edit {
  margin-top: var(--sp-3);
  min-height: 44px;
  padding: 10px 18px;
}
.stop {
  margin-top: var(--sp-2);
  padding-left: 0;
  min-height: 44px;
}
</style>
