<script setup lang="ts">
import type { Template } from '~/utils/periods'

const props = defineProps<{ templates: Template[], loading?: boolean }>()
const emit = defineEmits<{ stop: [template: Template] }>()

// stopping can't be undone, so the first click only asks
const confirming = ref<string>()
function stop(item: Template) {
  if (confirming.value !== item.id) {
    confirming.value = item.id
    return
  }
  confirming.value = undefined
  emit('stop', item)
}
const { isAdmin } = useRole()

// soonest next due date first, stopped ones (no next date) at the end
const sorted = computed(() => [...props.templates].sort((a, b) =>
  (a.nextDue ? fromRu(a.nextDue) : '9999').localeCompare(b.nextDue ? fromRu(b.nextDue) : '9999')))

const { visible, hasMore, sentinel } = useLazyList(sorted)
</script>

<template>
  <div class="table" :class="{ admin: isAdmin }">
    <div class="row head">
      <span class="mono muted">№</span>
      <span>Название</span>
      <span class="right">Сумма</span>
      <span class="currency">Валюта</span>
      <span>Периодичность</span>
      <span>Ответственный</span>
      <span>Следующий срок</span>
      <span>Статус</span>
      <span v-if="isAdmin" />
    </div>

    <!-- grey rows of the same size until the payments arrive -->
    <template v-if="loading">
      <div v-for="n in 4" :key="`s${n}`" class="row">
        <span />
        <span class="stack"><UiSkeleton width="150px" /><UiSkeleton width="90px" height="10px" /></span>
        <UiSkeleton width="90px" class="end" />
        <UiSkeleton width="30px" height="10px" class="currency-skeleton" />
        <span class="stack"><UiSkeleton width="90px" /><UiSkeleton width="60px" height="10px" /></span>
        <UiSkeleton width="110px" />
        <UiSkeleton width="80px" />
        <UiSkeleton width="70px" height="20px" />
        <span v-if="isAdmin" />
      </div>
    </template>

    <div
      v-for="(item, i) in visible"
      :key="item.id"
      class="row rise"
      :class="{ ended: item.ended }"
      :style="{ viewTransitionName: `template-${item.id}`, '--i': i }"
    >
      <span class="mono muted small">{{ String(i + 1).padStart(2, '0') }}</span>
      <span>
        <span class="title">{{ item.title }}</span>
        <span class="small muted">{{ item.category }}</span>
      </span>
      <span class="mono strong right dim">{{ item.amount }}</span>
      <span class="mono small muted currency">{{ item.currency }}</span>
      <span>
        <span class="dim">{{ item.frequency }}</span>
        <span class="small muted">{{ item.day }}</span>
      </span>
      <PersonCell :person="item.who" />
      <span class="mono dim">{{ item.nextDue ?? '—' }}</span>
      <span v-if="item.ended" class="muted">Прекращён</span>
      <UiStatus v-else status="active" />
      <!-- «История» waits for the backend -->
      <template v-if="isAdmin">
        <button
          v-if="!item.ended"
          type="button"
          class="action"
          :class="{ confirm: confirming === item.id }"
          @click="stop(item)"
          @blur="confirming = undefined"
        >
          {{ confirming === item.id ? 'Точно прекратить?' : 'Прекратить' }}
        </button>
        <button v-else type="button" class="action text-action soon" disabled :title="soonHint">История</button>
      </template>
    </div>
    <div v-if="hasMore" ref="sentinel" class="sentinel" />
  </div>
</template>

<style scoped>
/* no box around it: rows run the full width, separated by hairlines.
   the negative margin lets hover fills reach a little past the text edge */
.table {
  position: relative;
  margin: 0 -12px;
  border-bottom: 1px solid var(--line);
  font-size: 13px;
}

.row:not(.head):hover {
  background: var(--subtle);
}

.row {
  display: grid;
  /* the spare width is shared between the columns, so the gaps stay even */
  grid-template-columns: 28px minmax(120px, 2.2fr) minmax(100px, 1fr) 56px minmax(100px, 1.2fr) minmax(116px, 1.4fr) minmax(104px, 1fr) minmax(84px, 1fr);
  column-gap: 12px;
  align-items: center;
  height: 56px;
  padding: 0 12px;
  border-top: 1px solid var(--line);
  white-space: nowrap;
}

/* a cell may get narrower than its text, long names end with «…» */
.row > * {
  min-width: 0;
}

.admin .row {
  grid-template-columns: 28px minmax(120px, 2.2fr) minmax(100px, 1fr) 56px minmax(100px, 1.2fr) minmax(116px, 1.4fr) minmax(104px, 1fr) minmax(84px, 1fr) minmax(124px, 1fr);
}

/* header: plain small text, the hairline under it does the separating.
   above the rows, so a filter menu isn't hidden under them */
.row.head {
  position: relative;
  z-index: 4;
  height: 40px;
  border-top: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
}

.title {
  display: block;
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
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

/* stopped payment: everything grey except who pays */
.ended .title,
.ended .dim {
  color: var(--ink-3);
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.end {
  justify-self: end;
}

.currency-skeleton {
  margin-left: 20px;
}

.sentinel {
  height: 1px;
}

.action {
  background: none;
  border: 0;
  padding: 0;
  font-size: 13px;
  color: var(--ink-2);
  text-align: right;
  justify-self: end;
}

.action.confirm {
  color: var(--red);
  font-weight: 500;
}
</style>
