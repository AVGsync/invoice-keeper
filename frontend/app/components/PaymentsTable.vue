<script setup lang="ts">
import type { CurrencyTotal, Period } from '~/utils/periods'

const props = withDefaults(defineProps<{
  periods: Period[]
  totals: CurrencyTotal[]
  filtered?: boolean
  month?: string
  loading?: boolean
}>(), { month: 'октябрь' })
defineEmits<{ reset: [] }>()


const payments = usePayments()
const { visible, hasMore, sentinel } = useLazyList(() => props.periods)

// the row whose popover is open: marking it paid, or its receipts
const open = ref<{ id: string, kind: 'paid' | 'receipts' }>()
const isOpen = (period: Period, kind: 'paid' | 'receipts') => open.value?.id === period.id && open.value.kind === kind

// where the popover sits inside the row: its top left corner at the click
const at = ref({ x: 0, y: 0 })

// a paid row opens too: to fix the date or take the mark off
function toggle(period: Period, e: MouseEvent) {
  if (clickable(period)) openAt(period, 'paid', e, e.currentTarget as HTMLElement)
}

// the paperclip in the row: its receipts, without opening «mark paid»
function showReceipts(period: Period, e: MouseEvent) {
  const row = (e.currentTarget as HTMLElement).closest<HTMLElement>('.row')
  if (row) openAt(period, 'receipts', e, row)
}

async function openAt(period: Period, kind: 'paid' | 'receipts', e: MouseEvent, row: HTMLElement) {
  if (isOpen(period, kind)) {
    open.value = undefined
    return
  }
  const box = row.getBoundingClientRect()
  at.value = { x: e.clientX - box.left, y: e.clientY - box.top }
  open.value = { id: period.id, kind }

  // near the right or bottom edge of the window it opens to the other side of the click
  await nextTick()
  const popover = row.querySelector<HTMLElement>('.popover')
  if (!popover) return
  const gap = 12
  if (e.clientX + popover.offsetWidth + gap > window.innerWidth) at.value.x -= popover.offsetWidth
  if (e.clientY + popover.offsetHeight + gap > window.innerHeight) at.value.y -= popover.offsetHeight
}

const clickable = (period: Period) => period.status !== 'ended' && payments.canMark(period)

// receipts: the assignee and the admin see them. the rows on screen ask how many there are, a few at a time
const receipts = useReceipts()
watch(visible, rows => rows.filter(payments.canMark).forEach(period => receipts.ensure(period.id)), { immediate: true })
const receiptCount = (period: Period) => receipts.byPeriod.value[period.id]?.length
</script>

<template>
  <div class="table">
    <div class="row head">
      <span class="mono muted">№</span>
      <span>Название</span>
      <span>Ответственный</span>
      <span>Срок</span>
      <span class="right">Сумма</span>
      <span class="currency">Валюта</span>
      <span>Статус</span>
      <span>Дата оплаты</span>
      <span class="right">Просрочка</span>
      <span class="center">Чек</span>
    </div>

    <div
      v-for="(period, i) in visible"
      :key="period.id"
      class="row rise"
      :class="{ overdue: period.status === 'overdue', clickable: clickable(period), 'has-popover': open?.id === period.id }"
      :style="{ viewTransitionName: `row-${period.id}`, '--i': i }"
      @click="toggle(period, $event)"
    >
      <span class="mono muted small">{{ String(i + 1).padStart(2, '0') }}</span>
      <span>
        <span class="title">{{ period.title }}</span>
        <span class="small muted">{{ period.frequency }}</span>
      </span>
      <PersonCell :person="period.who" />
      <span>
        <span class="mono">{{ period.due }}</span>
        <span
          v-if="period.relative"
          class="small"
          :class="period.status === 'overdue' ? 'red' : period.relative === 'завтра' ? '' : 'muted'"
        >{{ period.relative }}</span>
      </span>
      <span class="mono strong right">{{ period.amount }}</span>
      <span class="mono small muted currency">{{ period.currency }}</span>
      <UiStatus :status="period.status" />
      <span class="mono" :class="{ muted: !period.paidAt }">{{ period.paidAt ?? '—' }}</span>
      <span class="mono right" :class="period.status === 'overdue' ? 'red strong' : period.status === 'late' ? 'amber strong' : 'muted'">
        {{ period.overdue ?? '—' }}
      </span>
      <span class="center">
        <button
          v-if="payments.canMark(period)"
          type="button"
          class="receipts"
          :class="{ has: receiptCount(period), opened: isOpen(period, 'receipts') }"
          :aria-label="receiptCount(period) ? `Чеки: ${receiptCount(period)}` : 'Прикрепить чек'"
          :title="receiptCount(period) ? 'Чеки: открыть или скачать' : 'Прикрепить чек'"
          @click.stop="showReceipts(period, $event)"
        >
          <UiIcon name="clip" :size="14" />
          <span v-if="receiptCount(period)" class="mono">{{ receiptCount(period) }}</span>
        </button>
        <span v-else class="muted">—</span>
      </span>

      <Transition name="menu">
        <MarkPaidPopover
          v-if="isOpen(period, 'paid')"
          :period="period"
          class="popover"
          :style="{ left: `${at.x}px`, top: `${at.y}px` }"
          @close="open = undefined"
        />
      </Transition>
      <Transition name="menu">
        <ReceiptsPopover
          v-if="isOpen(period, 'receipts')"
          :period="period"
          :can-edit="payments.canMark(period)"
          class="popover"
          :style="{ left: `${at.x}px`, top: `${at.y}px` }"
          @close="open = undefined"
        />
      </Transition>
    </div>

    <div v-if="hasMore" ref="sentinel" class="sentinel" />

    <!-- nothing matched: say so and offer a way out -->
    <template v-if="loading">
      <div v-for="n in 4" :key="n" class="row">
        <span /><span class="skeleton-stack"><UiSkeleton width="160px" /><UiSkeleton width="90px" height="10px" /></span>
        <UiSkeleton width="100px" /><UiSkeleton width="80px" /><UiSkeleton width="90px" class="right-skeleton" />
      </div>
    </template>
    <UiState v-else-if="!periods.length" class="empty">
      {{ filtered ? 'Под эти фильтры нет ни одного платежа.' : 'Платежей пока нет.' }}
      <template v-if="filtered">
        <br>
        <button type="button" class="reset" @click="$emit('reset')">Сбросить фильтры</button>
      </template>
    </UiState>

    <div v-else class="foot">
      <span>
        <span class="title">Итого за {{ month }}</span>
        <span class="small muted">по каждой валюте отдельно</span>
      </span>
      <span v-for="total in totals" :key="total.code" class="foot-total">
        <span class="mono small soft strong">{{ total.code }}</span>
        <span>
          <span class="mono strong">{{ total.total }}</span>
          <span class="mono small muted">оплачено {{ total.paid }}</span>
        </span>
      </span>
    </div>
  </div>
</template>

<style scoped>
/* filter changes: the table resizes, rows slide to their new place (useViewTransition) */
/* no box around it: rows run the full width, separated by hairlines.
   the negative margin lets hover fills reach a little past the text edge */
.table {
  view-transition-name: payments-table;
  position: relative;
  margin: 0 -12px;
  font-size: 13px;
}

.row {
  position: relative;
  display: grid;
  /* every column has its least width and a share of what's left, so the table fills the page evenly */
  grid-template-columns:
    28px minmax(104px, 2.2fr) minmax(112px, 1.4fr) minmax(88px, 1fr) minmax(104px, 1.1fr)
    56px minmax(176px, 1.6fr) minmax(84px, 1fr) minmax(68px, 0.8fr) 52px;
  column-gap: 12px;
  align-items: center;
  height: 56px;
  padding: 0 12px;
  border-top: 1px solid var(--line);
}

/* a cell may get narrower than its text, long names end with «…» */
.row > * {
  min-width: 0;
}

/* header: plain small text, the hairline under it does the separating.
   above the rows, so a filter menu isn't hidden under them */
.row.head {
  z-index: 4;
  height: 40px;
  border-top: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
}

/* every row is its own layer (view-transition-name), so the one with the popover
   has to sit above the rows and footer below it */
.row.has-popover {
  z-index: 3;
}

.row.clickable {
  cursor: pointer;
  -webkit-user-select: none;
  user-select: none;
}

.row.clickable:hover {
  background: var(--subtle);
}

.title {
  display: block;
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.small {
  display: block;
  font-size: 12px;
}

.strong {
  font-weight: 500;
}

.currency {
  padding-left: 8px;
}

.red { color: var(--red); }
.amber { color: var(--amber); }

/* opens where the row was clicked (left / top set in toggle), grows out of that corner */
/* the paperclip: faint when there are no receipts, coloured with the count when there are */
.receipts {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 26px;
  padding: 0 8px;
  background: none;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 12px;
  color: var(--ink-3);
  transition: background-color 0.2s ease, color 0.2s ease, border-color 0.2s ease, transform 0.35s var(--ease-spring);
}

.receipts:hover,
.receipts.opened {
  background: var(--surface);
  border-color: var(--line-strong);
  color: var(--ink);
  transform: rotate(-6deg);
}

.receipts.has {
  background: var(--accent-bg);
  color: var(--accent-ink);
}

.center {
  justify-self: center;
}

.popover {
  position: absolute;
  z-index: 2;
  cursor: default;
  transform-origin: top left;
}

.sentinel {
  height: 1px;
}

.reset {
  margin-top: 10px;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
}

.foot {
  view-transition-name: payments-foot;
  display: flex;
  align-items: center;
  height: 63px;
  padding: 0 12px;
  border-top: 1px solid var(--line-strong);
  border-bottom: 1px solid var(--line);
}

.foot > span:first-child {
  width: 180px;
  font-size: 13px;
}

.foot > span:first-child .title {
  font-size: 13px;
}

.foot-total {
  display: flex;
  gap: 12px;
  width: 220px;
}

.skeleton-stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.right-skeleton {
  justify-self: end;
}
</style>
