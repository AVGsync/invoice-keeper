<!-- month as 'YYYY-MM'. clicking the month name opens a 12-month dropdown, the small control also has arrows -->
<script setup lang="ts">
const props = withDefaults(defineProps<{
  // control = small box for filter rows, title = big month name like on Календарь
  variant?: 'control' | 'title'
  // 'YYYY-MM' to outline in the dropdown, usually the month of "today"
  current?: string
}>(), { variant: 'control', current: undefined })

const model = defineModel<string>({ required: true })

const root = ref<HTMLElement>()
const open = ref(false)
useClickOutside(root, () => { open.value = false }, open)

const shown = computed(() => parseMonthKey(model.value))
const currentMonth = computed(() => (props.current ? parseMonthKey(props.current) : undefined))

function set(year: number, month: number) {
  model.value = monthKey(year, month)
  open.value = false
}

function step(by: number) {
  set(shown.value.year, shown.value.month + by)
}
</script>

<template>
  <div ref="root" class="month-picker" :class="variant">
    <template v-if="variant === 'title'">
      <button type="button" class="label title-label" :aria-expanded="open" @click="open = !open">
        <span class="display big">{{ monthNames[shown.month] }}</span>
        <span class="mono year">{{ shown.year }}</span>
        <!-- the chevron says «this opens something», like on the dropdowns -->
        <UiIcon name="down" :size="15" class="caret" :class="{ flip: open }" />
      </button>
    </template>

    <template v-else>
      <button type="button" class="arrow" aria-label="Предыдущий месяц" @click="step(-1)"><UiIcon name="left" :size="15" /></button>
      <button type="button" class="label" :aria-expanded="open" @click="open = !open">
        {{ monthNames[shown.month] }} {{ shown.year }}
      </button>
      <button type="button" class="arrow" aria-label="Следующий месяц" @click="step(1)"><UiIcon name="right" :size="15" /></button>
    </template>

    <Transition name="menu">
      <UiMonthGrid
        v-if="open"
        class="dropdown"
        :year="shown.year"
        :selected="shown"
        :current="currentMonth"
        @pick="set"
      />
    </Transition>
  </div>
</template>

<style scoped>
.month-picker {
  position: relative;
  display: inline-flex;
  align-items: center;
}

/* small box, same look as the old month switcher */
.control {
  justify-content: space-between;
  width: 196px;
  height: 32px;
  padding: 0 6px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 9999px;
}

.control .label {
  flex: 1;
  height: 100%;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
}

.arrow {
  display: inline-grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 9999px;
  background: none;
  border: 0;
  font-size: 15px;
  color: var(--ink-2);
}

/* a ghost button: reads as the page's heading, a soft fill on hover and while open shows it's clickable.
   the negative margin keeps the text where it was */
.title-label {
  display: inline-flex;
  align-items: baseline;
  gap: 10px;
  margin-left: -12px;
  padding: 2px 12px;
  background: none;
  border: 0;
  border-radius: 9999px;
  transition: background-color 0.2s ease;
}

.title-label:hover,
.title-label[aria-expanded="true"] {
  background: var(--subtle);
}

.caret {
  align-self: center;
  margin-left: -4px;
  color: var(--ink-3);
  transition: transform 0.35s var(--ease-out), color 0.2s ease;
}

.title-label:hover .caret {
  color: var(--ink);
}

.caret.flip {
  transform: rotate(180deg);
}

.big {
  font-size: 24px;
  line-height: 32px;
}

.year {
  font-size: 15px;
  color: var(--ink-3);
}

.arrow:hover {
  background: var(--subtle);
  color: var(--ink);
}

.dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 5;
  width: 240px;
  padding: 10px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--shadow-pop);
}
</style>
