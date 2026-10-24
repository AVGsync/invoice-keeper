<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { occurrences, occState } from '../stores/occurrences'
import { session } from '../stores/session'
import { confirmOccurrence, cancelOccurrence, getOccurrence } from '../api/endpoints'
import { ApiError } from '../api/errors'
import { canMarkPaid } from '../lib/permissions'
import { formatDate, formatShort, relativeDay, today } from '../lib/dates'
import { humanError } from '../lib/humanError'
import { haptic, confirm as tgConfirm, alert as tgAlert } from '../telegram/webapp'
import { useMainButton } from '../composables/useMainButton'
import Money from '../components/Money.vue'
import StatusBadge from '../components/StatusBadge.vue'
import Section from '../components/Section.vue'
import Cell from '../components/Cell.vue'
import Skeleton from '../components/Skeleton.vue'
import type { PaymentResponse } from '../api/types'

const route = useRoute()
const router = useRouter()

const occ = ref<PaymentResponse | null>(null)
const loading = ref(false)
const notFound = ref(false)
const busy = ref(false)
const payDate = ref(today())
const editingDate = ref(false)
let controller: AbortController | null = null

const todayStr = computed(() => today())

async function loadOcc(id: string): Promise<void> {
  notFound.value = false
  const cached = occurrences.byId(id)
  if (cached) {
    occ.value = cached
    payDate.value = cached.paid_date || today()
    return
  }
  loading.value = true
  controller?.abort()
  controller = new AbortController()
  try {
    const data = await getOccurrence(id, controller.signal)
    occ.value = data
    payDate.value = data.paid_date || today()
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') return
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) notFound.value = true
    else notFound.value = true
  } finally {
    loading.value = false
  }
}

watch(
  () => route.params.id,
  (id) => {
    if (typeof id === 'string') void loadOcc(id)
  },
  { immediate: true },
)

// Если список обновился в сторе, подхватываем свежую версию.
watch(
  () => occState.items,
  () => {
    const id = route.params.id
    if (typeof id === 'string') {
      const fresh = occurrences.byId(id)
      if (fresh) occ.value = fresh
    }
  },
  { deep: true },
)

const canPay = computed(() => (occ.value ? canMarkPaid(session.user, occ.value) : false))
const isPayable = computed(
  () => occ.value?.status === 'pending' || occ.value?.status === 'overdue',
)
const isPaid = computed(() => occ.value?.status === 'paid')
const isEnded = computed(() => occ.value?.status === 'ended')

// MainButton активна только для оплачиваемых периодов с правами.
const mainVisible = computed(() => canPay.value && isPayable.value)
// Дата должна быть корректной YYYY-MM-DD и не в будущем.
const payDateValid = computed(
  () => /^\d{4}-\d{2}-\d{2}$/.test(payDate.value) && payDate.value <= todayStr.value,
)
const mainText = computed(() => {
  if (!payDateValid.value) return 'Выберите дату'
  return payDate.value === todayStr.value
    ? 'Оплачено сегодня'
    : 'Оплачено ' + formatShort(payDate.value)
})

async function doConfirm(paidDate: string | undefined): Promise<void> {
  if (!occ.value || busy.value) return
  busy.value = true
  try {
    await confirmOccurrence(occ.value.id, paidDate)
    haptic.success()
    await occurrences.refreshOne(occ.value.id)
    editingDate.value = false
  } catch (e) {
    haptic.error()
    if (e instanceof ApiError && e.status === 409) {
      await occurrences.refreshOne(occ.value.id).catch(() => {})
      await tgAlert('Платёж уже отмечен оплаченным')
    } else {
      await tgAlert(humanError(e))
    }
  } finally {
    busy.value = false
  }
}

function payNow(): void {
  if (!payDateValid.value) return
  const d = payDate.value === todayStr.value ? undefined : payDate.value
  void doConfirm(d)
}

async function saveDate(): Promise<void> {
  if (!payDateValid.value) return
  await doConfirm(payDate.value)
}

async function cancelPayment(): Promise<void> {
  if (!occ.value || busy.value) return
  const ok = await tgConfirm('Снять отметку об оплате?')
  if (!ok) return
  busy.value = true
  try {
    await cancelOccurrence(occ.value.id)
    haptic.success()
    await occurrences.refreshOne(occ.value.id)
  } catch (e) {
    haptic.error()
    await tgAlert(humanError(e))
  } finally {
    busy.value = false
  }
}

useMainButton({
  text: mainText,
  visible: mainVisible,
  enabled: computed(() => payDateValid.value && !busy.value),
  loading: busy,
  onClick: payNow,
})

const history = computed(() => (occ.value ? occurrences.history(occ.value) : []))

function openOcc(id: string): void {
  router.push(`/occ/${id}`)
}
function goHome(): void {
  router.replace('/')
}

onUnmounted(() => controller?.abort())
</script>

<template>
  <div class="occ">
    <Skeleton v-if="loading && !occ" :rows="4" />

    <div v-else-if="notFound" class="notfound">
      <p>Платёж не найден или удалён</p>
      <button class="btn" type="button" @click="goHome">На главную</button>
    </div>

    <template v-else-if="occ">
      <div class="head">
        <div class="desc">{{ occ.description }}</div>
        <Money :amount="occ.amount" :currency="occ.currency" large />
        <div class="badge"><StatusBadge :status="occ.status" :paid-late="occ.paid_late" /></div>
      </div>

      <Section>
        <Cell label="Срок оплаты">
          {{ formatDate(occ.due_date, { today: todayStr }) }}
          <span v-if="isPayable" class="hint"> · {{ relativeDay(occ.due_date, todayStr) }}</span>
        </Cell>
        <Cell v-if="occ.assignee_name" label="Ответственный" :value="occ.assignee_name" />
        <Cell v-if="occ.paid_date" label="Оплачено">
          {{ formatDate(occ.paid_date, { today: todayStr }) }}
          <span v-if="occ.paid_late" class="late"> · с опозданием</span>
        </Cell>
        <Cell v-if="occ.recurrent_payment_id" label="Регулярный платёж" value="Да" />
      </Section>

      <!-- Оплачиваемый период: выбор даты оплаты -->
      <Section v-if="mainVisible" title="Дата оплаты">
        <div class="date-row">
          <input v-model="payDate" type="date" :max="todayStr" />
        </div>
      </Section>

      <!-- Оплачено: изменить дату / отменить -->
      <Section v-else-if="canPay && isPaid">
        <div v-if="!editingDate" class="actions">
          <button class="btn btn-secondary" type="button" @click="editingDate = true">
            Изменить дату оплаты
          </button>
          <button class="btn btn-destructive" type="button" :disabled="busy" @click="cancelPayment">
            Отменить оплату
          </button>
        </div>
        <div v-else class="date-edit">
          <input v-model="payDate" type="date" :max="todayStr" />
          <button class="btn" type="button" :disabled="busy || !payDateValid" @click="saveDate">
            Сохранить дату
          </button>
          <button class="btn btn-ghost" type="button" @click="editingDate = false">Отмена</button>
        </div>
      </Section>

      <!-- Остановленный платёж -->
      <Section v-else-if="isEnded">
        <div class="banner">Платёж остановлен — новых напоминаний не будет</div>
        <div v-if="canPay" class="actions">
          <template v-if="!editingDate">
            <button class="btn btn-ghost" type="button" @click="editingDate = true">
              Всё равно отметить оплату
            </button>
          </template>
          <template v-else>
            <input v-model="payDate" type="date" :max="todayStr" />
            <button class="btn" type="button" :disabled="busy || !payDateValid" @click="saveDate">
              Отметить оплату
            </button>
            <button class="btn btn-ghost" type="button" @click="editingDate = false">Отмена</button>
          </template>
        </div>
      </Section>

      <!-- Нет прав -->
      <Section v-else-if="!canPay">
        <p class="hint hint-pad">
          Отметить оплату может ответственный ({{ occ.assignee_name || '—' }}) или администратор
        </p>
      </Section>

      <Section v-if="history.length" title="История">
        <button
          v-for="h in history"
          :key="h.id"
          type="button"
          class="hrow row-active"
          @click="openOcc(h.id)"
        >
          <span>{{ formatDate(h.due_date, { today: todayStr }) }}</span>
          <StatusBadge :status="h.status" :paid-late="h.paid_late" />
          <Money :amount="h.amount" :currency="h.currency" />
        </button>
      </Section>
    </template>
  </div>
</template>

<style scoped>
.head {
  padding: var(--sp-6) 0 var(--sp-2);
}
.desc {
  font-size: var(--fs-body-l);
  line-height: 1.3;
  font-weight: 500;
  margin-bottom: var(--sp-3);
  color: var(--ash-text);
}
.head > .badge {
  margin-top: var(--sp-4);
}
.late:not(.badge) {
  color: var(--bone);
}
.hint-pad {
  padding: var(--sp-4) 0;
  margin: 0;
}
.date-row,
.date-edit {
  padding: var(--sp-4) 0;
  display: flex;
  flex-direction: column;
  gap: var(--sp-3);
}
.actions {
  padding: var(--sp-4) 0;
  display: flex;
  flex-direction: column;
  gap: var(--sp-3);
}
.actions .btn,
.date-edit .btn {
  width: 100%;
}
.banner {
  padding: var(--sp-4) 0;
  color: var(--ash-text);
  font-size: var(--fs-body);
}
.notfound {
  text-align: center;
  padding: var(--sp-9) 0;
}
.notfound p {
  font-size: var(--fs-body-l);
  margin-bottom: 16px;
}
.hrow {
  display: grid;
  grid-template-columns: 1fr auto;
  column-gap: var(--sp-3);
  row-gap: 6px;
  align-items: center;
  text-align: left;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--bone);
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.hrow > :nth-child(2) {
  grid-row: 2;
  grid-column: 1;
  justify-self: start;
}
.hrow > :nth-child(3) {
  grid-row: 1;
  grid-column: 2;
}
</style>
