<!-- «Журнал»: who did what and when. the admin sees everybody (GET /audit-log),
     a user sees only their own actions (GET /me/activity). filters are pills above the table,
     the next page comes when the end of the list scrolls into view, a click on a row opens its details -->
<script setup lang="ts">
import type { AuditActionInfo, AuditLogItem, AuditLogParams, User } from '~/types/api'

const props = defineProps<{ mine?: boolean }>()

const api = useApi()
const PAGE = 50

// the filter pills, '' = off
const filters = reactive({ from: '', to: '', actor: '', action: '', entity: '', channel: '', outcome: '', q: '' })
const filtered = computed(() => Object.values(filters).some(Boolean))

const actions = ref<AuditActionInfo[]>([])
const users = ref<User[]>([])
onMounted(async () => {
  actions.value = (await api.audit.actions().catch(() => ({ items: [] }))).items ?? []
  if (!props.mine) users.value = (await api.users.list().catch(() => ({ items: [] }))).items ?? []
})

const actionOptions = computed(() => actions.value.map(item => ({ value: item.action ?? '', label: actionLabel(item.label, item.action) })))
const actorOptions = computed(() => users.value.map(user => ({ value: user.id ?? '', label: user.full_name ?? '', meta: roleName(user.role) })))
const entityOptions = Object.entries(entityNames).map(([value, label]) => ({ value, label }))
const channelOptions = Object.entries(channelNames).map(([value, label]) => ({ value, label }))
const outcomeOptions = [{ value: 'success', label: 'Успешно' }, { value: 'failure', label: 'Ошибка' }]

const params = computed<AuditLogParams>(() => ({
  actor_id: !props.mine && filters.actor ? [filters.actor] : undefined,
  action: filters.action ? [filters.action] : undefined,
  entity_type: filters.entity ? [filters.entity] : undefined,
  channel: filters.channel ? [filters.channel] : undefined,
  outcome: (filters.outcome || undefined) as AuditLogParams['outcome'],
  from: filters.from || undefined,
  to: filters.to || undefined,
  q: filters.q.trim() || undefined,
  order: 'desc',
}))

const items = ref<AuditLogItem[]>([])
const total = ref(0)
const loading = ref(true)
const loadingMore = ref(false)
const error = ref('')
let latest = 0
let timer: ReturnType<typeof setTimeout> | undefined

const fetchPage = (offset: number) => {
  const query = { ...params.value, limit: PAGE, offset }
  return props.mine ? api.audit.mine(query) : api.audit.list(query)
}

// a new set of filters: start from the newest entries again. typing waits a moment
async function reload() {
  const mine = ++latest
  loading.value = true
  error.value = ''
  try {
    const page = await fetchPage(0)
    if (mine !== latest) return
    items.value = page.items ?? []
    total.value = page.total ?? 0
  }
  catch (e) {
    if (mine === latest) error.value = apiError(e)
  }
  finally {
    if (mine === latest) loading.value = false
  }
}

watch(params, () => {
  clearTimeout(timer)
  timer = setTimeout(reload, 300)
}, { deep: true })
onMounted(reload)
onBeforeUnmount(() => clearTimeout(timer))

const hasMore = computed(() => items.value.length < total.value)
async function more() {
  if (loadingMore.value || loading.value || !hasMore.value) return
  const mine = latest
  loadingMore.value = true
  try {
    const page = await fetchPage(items.value.length)
    if (mine === latest) items.value.push(...(page.items ?? []))
  }
  catch (e) {
    if (mine === latest) error.value = apiError(e)
  }
  finally {
    loadingMore.value = false
  }
}

// the next page when the end of the list comes near
const sentinel = ref<HTMLElement>()
let observer: IntersectionObserver | undefined
onMounted(() => {
  observer = new IntersectionObserver(entries => entries.some(entry => entry.isIntersecting) && more(), { rootMargin: '300px' })
})
watch(sentinel, (el) => {
  observer?.disconnect()
  if (el) observer?.observe(el)
}, { flush: 'post' })
onBeforeUnmount(() => observer?.disconnect())

// new entries show up on their own: every few seconds we ask for the newest page
// and put whatever is new on top. nothing is asked while the tab is hidden
const fresh = ref(new Set<number | undefined>())
let polling = false
async function poll() {
  if (polling || loading.value || error.value || document.hidden) return
  const mine = latest
  polling = true
  try {
    const page = await fetchPage(0)
    if (mine !== latest) return
    const newest = page.items ?? []
    const known = new Set(items.value.map(item => item.id))
    const added = newest.filter(item => !known.has(item.id))
    if (!added.length) return
    // a whole page of new ones: the gap can't be stitched, start from the top
    if (added.length === newest.length && items.value.length) items.value = newest
    else items.value = [...added, ...items.value]
    total.value = page.total ?? total.value
    // new rows light up for a moment
    added.forEach(item => fresh.value.add(item.id))
    setTimeout(() => added.forEach(item => fresh.value.delete(item.id)), 2400)
  }
  catch {
    // a missed round is fine, the next one tries again
  }
  finally {
    polling = false
  }
}

let ticker: ReturnType<typeof setInterval> | undefined
// back to the tab: check right away instead of waiting for the next round
const onVisible = () => !document.hidden && poll()
onMounted(() => {
  ticker = setInterval(poll, 8000)
  document.addEventListener('visibilitychange', onVisible)
})
onBeforeUnmount(() => {
  clearInterval(ticker)
  document.removeEventListener('visibilitychange', onVisible)
})

function reset() {
  Object.assign(filters, { from: '', to: '', actor: '', action: '', entity: '', channel: '', outcome: '', q: '' })
}

// the row whose details are open
const open = ref<number>()
const toggle = (item: AuditLogItem) => { open.value = open.value === item.id ? undefined : item.id }

const when = (iso?: string) => (iso
  ? new Date(iso).toLocaleString('ru-RU', { timeZone: 'Europe/Moscow', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }).replace(',', '')
  : '')
const actorName = (item: AuditLogItem) => item.actor?.full_name || (item.channel === 'system' ? 'Система' : 'Удалённый пользователь')
// the first change or, without changes, what the entry was about: one line in the row
const summary = (item: AuditLogItem) => changeLines(item)[0] ?? payloadLines(item)[0] ?? ''
const extra = (item: AuditLogItem) => Math.max(0, changeLines(item).length - 1)
</script>

<template>
  <div>
    <!-- the filters, as pills above the table like on the other pages -->
    <div class="filters">
      <FilterPill v-model:from="filters.from" v-model:to="filters.to" label="Когда" range="date" />
      <FilterPill v-if="!mine" v-model="filters.actor" label="Кто" :options="actorOptions" />
      <FilterPill v-model="filters.action" label="Действие" :options="actionOptions" />
      <FilterPill v-model="filters.entity" label="Объект" :options="entityOptions" />
      <FilterPill v-model="filters.q" label="Подробности" />
      <FilterPill v-model="filters.channel" label="Откуда" :options="channelOptions" />
      <FilterPill v-model="filters.outcome" label="Итог" :options="outcomeOptions" />
    </div>

    <div class="card table" :class="{ mine }">
      <div class="row head">
        <span>Когда</span>
        <span v-if="!mine">Кто</span>
        <span>Действие</span>
        <span>Объект</span>
        <span>Подробности</span>
        <span>Откуда</span>
        <span class="end">Итог</span>
      </div>

      <!-- grey rows of the same size until the entries arrive -->
      <template v-if="loading">
        <div v-for="n in 6" :key="n" class="row">
          <UiSkeleton width="110px" />
          <span v-if="!mine" class="stack"><UiSkeleton width="120px" /><UiSkeleton width="70px" height="10px" /></span>
          <UiSkeleton width="150px" />
          <UiSkeleton width="110px" />
          <UiSkeleton width="200px" />
          <UiSkeleton width="60px" height="20px" />
          <UiSkeleton width="70px" height="20px" class="end" />
        </div>
      </template>

      <template v-else>
        <template v-for="(item, i) in items" :key="item.id">
          <div class="row entry rise" :class="{ opened: open === item.id, fresh: fresh.has(item.id) }" :style="{ '--i': i % PAGE }" @click="toggle(item)">
            <span class="mono soft nowrap">{{ when(item.created_at) }}</span>
            <span v-if="!mine">
              <span class="name">{{ actorName(item) }}</span>
              <span v-if="item.actor?.role" class="small muted">{{ roleName(item.actor.role) }}</span>
            </span>
            <span class="strong">{{ actionLabel(item.action_label, item.action) }}</span>
            <span class="soft">{{ entityNames[item.entity_type ?? ''] ?? '—' }}</span>
            <span class="summary">
              <span class="ellipsis">{{ summary(item) || '—' }}</span>
              <span v-if="extra(item)" class="small muted">ещё {{ extra(item) }}</span>
            </span>
            <span><span class="pill">{{ channelNames[item.channel ?? ''] ?? item.channel }}</span></span>
            <span class="end">
              <span class="outcome" :class="item.outcome"><span class="dot" />{{ item.outcome === 'failure' ? 'Ошибка' : 'Успешно' }}</span>
            </span>
          </div>

          <!-- everything the entry has: what changed (was → now), what it was about, where it came from -->
          <Transition name="unfold">
            <div v-if="open === item.id" class="unfold">
              <div class="clip">
                <div class="details">
                  <div v-if="changeLines(item).length" class="block">
                    <p class="kicker">Изменения</p>
                    <p v-for="line in changeLines(item)" :key="line">{{ line }}</p>
                  </div>
                  <div v-if="payloadLines(item).length" class="block">
                    <p class="kicker">Подробности</p>
                    <p v-for="line in payloadLines(item)" :key="line">{{ line }}</p>
                  </div>
                  <div class="block">
                    <p class="kicker">Откуда</p>
                    <p v-if="item.ip" class="mono">IP {{ item.ip }}</p>
                    <p v-if="item.user_agent" class="muted small-text">{{ item.user_agent }}</p>
                    <p v-if="item.request_id" class="mono muted small-text">запрос {{ item.request_id }}</p>
                    <p v-if="!item.ip && !item.user_agent && !item.request_id" class="muted">нет данных</p>
                  </div>
                </div>
              </div>
            </div>
          </Transition>
        </template>

        <div v-if="hasMore" ref="sentinel" class="row more">
          <UiSkeleton width="160px" />
        </div>

        <UiState v-if="error" kind="error" class="state" @retry="reload">Не удалось загрузить журнал: {{ error }}</UiState>
        <UiState v-else-if="!items.length" class="state">
          {{ filtered ? 'Под эти фильтры записей нет.' : mine ? 'Вы пока ничего не делали.' : 'Журнал пока пуст.' }}
          <template v-if="filtered">
            <br>
            <button type="button" class="reset" @click="reset">Сбросить фильтры</button>
          </template>
        </UiState>
      </template>
    </div>
  </div>
</template>

<style scoped>
/* a row that just arrived: comes up like the others and has a soft accent tint that fades out */
.row.fresh {
  animation: rise 0.6s var(--ease-out) backwards, fresh 2.4s var(--ease-out);
}

@keyframes fresh {
  from {
    background: var(--accent-bg);
  }
}

.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
}

/* a rounded card like the other admin tables, rows separated by hairlines inside */
.table {
  overflow: visible;
  font-size: 13px;
}

.row {
  display: grid;
  /* every column has its least width and a share of what's left */
  grid-template-columns:
    minmax(136px, 1fr) minmax(140px, 1.3fr) minmax(170px, 1.6fr) minmax(120px, 1.2fr)
    minmax(0, 2.2fr) minmax(84px, 0.8fr) minmax(90px, 0.8fr);
  align-items: center;
  column-gap: 12px;
  min-height: 56px;
  padding: 0 16px;
  border-top: 1px solid var(--line);
}

.mine .row {
  grid-template-columns:
    minmax(136px, 1fr) minmax(170px, 1.6fr) minmax(120px, 1.2fr)
    minmax(0, 2.2fr) minmax(84px, 0.8fr) minmax(90px, 0.8fr);
}

/* header: plain small text, the hairline under it does the separating */
.row.head {
  min-height: 40px;
  border-top: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
}

.entry {
  cursor: pointer;
  -webkit-user-select: none;
  user-select: none;
  transition: background-color 0.2s ease;
}

.entry:hover,
.entry.opened {
  background: var(--subtle);
}

.name {
  display: block;
  font-weight: 500;
}

.small {
  display: block;
  font-size: 12px;
}

.strong {
  font-weight: 500;
}

.summary {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.nowrap {
  white-space: nowrap;
}

.ellipsis {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary .small {
  flex-shrink: 0;
}

/* where it came from: a quiet grey pill */
.pill {
  display: inline-block;
  height: 22px;
  padding: 0 9px;
  border-radius: 9999px;
  background: var(--subtle);
  border: 1px solid var(--line);
  font-size: 11px;
  font-weight: 500;
  line-height: 20px;
  color: var(--ink-2);
}

.end {
  justify-self: end;
}

/* success / failure: coloured pill, like the payment statuses */
.outcome {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 9px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
}

.outcome .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.outcome.success { background: var(--green-bg); color: var(--green); }
.outcome.failure { background: var(--red-bg); color: var(--red); }

.details {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
  padding: 14px 16px 18px;
  background: var(--subtle);
  border-top: 1px dashed var(--line);
  font-size: 13px;
  line-height: 20px;
  -webkit-user-select: text;
  user-select: text;
}

.block .kicker {
  margin-bottom: 4px;
}

.small-text {
  font-size: 12px;
  overflow-wrap: anywhere;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.more {
  justify-content: center;
}

.state {
  border-top: 1px solid var(--line);
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
</style>
