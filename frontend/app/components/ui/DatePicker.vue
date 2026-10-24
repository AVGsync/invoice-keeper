<script setup lang="ts">
// value is 'YYYY-MM-DD', the field shows 'DD.MM.YYYY'. the date can only be picked in the calendar, not typed
const props = withDefaults(defineProps<{
  // which day is "today"
  today?: string
  // small grey word in front of the date, e.g. «С»
  prefix?: string
  placeholder?: string
  // looks like a menu row instead of a field, for the filter menus
  bare?: boolean
}>(), { today: undefined, prefix: '', placeholder: 'ДД.ММ.ГГГГ', bare: false })

const model = defineModel<string>({ default: '' })
// set by a surrounding UiField so its label points here
const fieldId = inject<string | undefined>('fieldId', undefined)

const root = ref<HTMLElement>()
const open = ref(false)

const today = computed(() => props.today ?? todayIso())

// month shown in the calendar
const view = ref(monthOf(model.value || today.value))
const cells = computed(() => monthCells(view.value.year, view.value.month))
const weekdays = ['ПН', 'ВТ', 'СР', 'ЧТ', 'ПТ', 'СБ', 'ВС']

function monthOf(iso: string) {
  const [year, month] = iso.split('-').map(Number) as [number, number]
  return { year, month: month - 1 }
}

// which way the month went, so the days slide in from that side
const way = ref<'next' | 'prev'>('next')

function shift(step: number) {
  way.value = step > 0 ? 'next' : 'prev'
  const date = new Date(view.value.year, view.value.month + step, 1)
  view.value = { year: date.getFullYear(), month: date.getMonth() }
}

// which way the calendar opens, so it never runs off the screen
const placement = ref({ right: false, up: false })
const CALENDAR = { width: 260, height: 330 }

// days = the usual grid, months = 12 months to jump to (click the month name)
const mode = ref<'days' | 'months'>('days')
const todayMonth = computed(() => monthOf(today.value))

function pickMonth(year: number, month: number) {
  view.value = { year, month }
  mode.value = 'days'
}

function openCalendar() {
  if (open.value) return
  view.value = monthOf(model.value || today.value)
  mode.value = 'days'
  const box = root.value!.getBoundingClientRect()
  placement.value = {
    right: box.left + CALENDAR.width > window.innerWidth - 16,
    up: box.bottom + CALENDAR.height > window.innerHeight - 16 && box.top > CALENDAR.height,
  }
  open.value = true
}

function toggle() {
  if (open.value) open.value = false
  else openCalendar()
}

function pick(iso: string) {
  model.value = iso
  open.value = false
}


useClickOutside(root, () => { open.value = false }, open)
</script>

<template>
  <div ref="root" class="picker">
    <!-- the whole field is one button: a click anywhere opens/closes the calendar.
         Enter and Space do the same, the down arrow only opens -->
    <button
      :id="fieldId"
      type="button"
      class="input mono"
      :class="{ open, bare }"
      :aria-expanded="open"
      @click="toggle"
      @keydown.down.prevent="openCalendar"
    >
      <span v-if="prefix" class="prefix">{{ prefix }}</span>
      <span class="value" :class="{ muted: !model }">{{ model ? toRu(model) : placeholder }}</span>
      <UiIcon name="down" :size="14" class="caret" :class="{ flip: open }" />
    </button>

    <Transition name="menu">
      <div v-if="open" class="calendar" :class="placement">
        <UiMonthGrid
          v-if="mode === 'months'"
          :year="view.year"
          :selected="view"
          :current="todayMonth"
          @pick="pickMonth"
        />

        <template v-else>
          <div class="head">
            <button type="button" class="arrow" aria-label="Предыдущий месяц" @click="shift(-1)"><UiIcon name="left" :size="15" /></button>
            <button type="button" class="month" title="Выбрать месяц" @click="mode = 'months'">
              {{ monthNames[view.month] }} {{ view.year }}
            </button>
            <button type="button" class="arrow" aria-label="Следующий месяц" @click="shift(1)"><UiIcon name="right" :size="15" /></button>
          </div>

          <Transition :name="`slide-${way}`" mode="out-in">
            <div :key="`${view.year}-${view.month}`" class="grid">
              <span v-for="name in weekdays" :key="name" class="weekday mono">{{ name }}</span>
              <button
                v-for="cell in cells"
                :key="cell.iso"
                type="button"
                class="day mono"
                :class="{ other: !cell.inMonth, weekend: cell.weekend, today: cell.iso === today, selected: cell.iso === model }"
                @click="pick(cell.iso)"
              >
                {{ cell.day }}
              </button>
            </div>
          </Transition>
        </template>

        <button type="button" class="today-link" @click="pick(today)">Сегодня, {{ toRu(today) }}</button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.picker {
  position: relative;
}

.input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 40px;
  padding: 0 14px 0 16px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 9999px;
  font-size: 14px;
  text-align: left;
  outline: none;
  user-select: none;
  transition: border-color 0.2s ease, box-shadow 0.35s var(--ease-out);
}

.input:hover:not(:focus, .open) {
  border-color: var(--ink-3);
}

.input:focus,
.input.open {
  border-color: var(--ink-2);
  box-shadow: 0 0 0 3px var(--subtle);
  animation: focus-ring 0.45s var(--ease-out);
}

.input.bare {
  gap: 8px;
  height: 34px;
  padding: 0 10px;
  background: none;
  border: 0;
  font-size: 13px;
  box-shadow: none;
  animation: none;
  transition: background-color 0.2s ease;
}

.input.bare:hover,
.input.bare.open,
.input.bare:focus-visible {
  background: var(--subtle);
}

.prefix {
  width: 22px;
  font-family: var(--sans);
  font-size: 12px;
  color: var(--ink-3);
}

.input.bare .value {
  margin-right: auto;
}

.input.bare .value.muted {
  font-family: var(--sans);
}

.caret {
  color: var(--ink-3);
  transition: transform 0.35s var(--ease-out);
}

.caret.flip {
  transform: rotate(180deg);
}

.calendar {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 5;
  width: 260px;
  padding: 10px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--shadow-pop);
}

.calendar.right {
  left: auto;
  right: 0;
}

.calendar.up {
  top: auto;
  bottom: calc(100% + 6px);
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.month {
  padding: 4px 8px;
  background: none;
  border: 0;
  border-radius: 9999px;
  font-size: 13px;
  font-weight: 500;
}

.month:hover {
  background: var(--subtle);
}

.arrow:hover {
  background: var(--subtle);
  color: var(--ink);
}

.arrow {
  display: inline-grid;
  place-items: center;
  width: 28px;
  height: 28px;
  background: none;
  border: 0;
  border-radius: 9999px;
  font-size: 15px;
  color: var(--ink-2);
}

.grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
}

.weekday {
  padding: 4px 0;
  font-size: 11px;
  text-align: center;
  color: var(--ink-3);
}

.day {
  height: 30px;
  background: none;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 12px;
}

.day:hover {
  background: var(--subtle);
}

.day.weekend {
  color: var(--ink-2);
}

.day.other {
  color: var(--ink-3);
  opacity: 0.6;
}

.day.today {
  border-color: var(--accent);
}

.day.selected {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.today-link {
  width: 100%;
  margin-top: 8px;
  padding: 6px 0 0;
  background: none;
  border: 0;
  border-top: 1px solid var(--line);
  font-size: 12px;
  color: var(--accent-ink);
}
</style>
