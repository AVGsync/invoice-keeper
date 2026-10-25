<!-- a filter pill for the bar above a table: «Сумма: от 1 000 ₽». a click opens a small menu with
     a text field (default), the values to pick from (`options`), or two rows «с / по» (`range`):
     dates are picked in the calendar, amounts are typed.
     '' means no filter, the cross on the pill takes it off -->
<script setup lang="ts">
import type { DropdownOption } from '~/components/ui/Dropdown.vue'

const props = withDefaults(defineProps<{
  label: string
  options?: DropdownOption[]
  // two fields: dates (YYYY-MM-DD) or amounts (in rubles / dollars / euros)
  range?: 'date' | 'amount'
  // the sign after the amount fields, e.g. ₽
  unit?: string
  // the filter can't be used right now, this says why (e.g. an amount needs one currency)
  blocked?: string
  // the menu opens to the left, for pills on the right edge
  right?: boolean
}>(), { options: undefined, range: undefined, unit: '', blocked: '', right: false })

const model = defineModel<string>({ default: '' })
const from = defineModel<string>('from', { default: '' })
const to = defineModel<string>('to', { default: '' })
const active = computed(() => !!(model.value || from.value || to.value))

const open = ref(false)
const root = ref<HTMLElement>()
useClickOutside(root, () => { open.value = false }, open)

// the first field is ready to type into as soon as the menu opens
watch(open, (value) => {
  if (value && !props.options) nextTick(() => root.value?.querySelector<HTMLInputElement>('.menu input')?.focus())
})

const current = computed(() => props.options?.find(option => option.value === model.value))

// what's picked, on the pill itself: «с 01.10 по 15.10», «от 1 000 ₽», «аренда»
const short = (value: string) => (props.range === 'date' ? toRu(value).slice(0, 5) : `${value} ${props.unit}`.trim())
const summary = computed(() => {
  if (props.range) {
    if (from.value && to.value && props.range === 'date') return `${short(from.value)} – ${short(to.value)}`
    return [from.value && `${props.range === 'date' ? 'с' : 'от'} ${short(from.value)}`, to.value && `${props.range === 'date' ? 'по' : 'до'} ${short(to.value)}`].filter(Boolean).join(' ')
  }
  if (props.options) return current.value?.label ?? ''
  return model.value ? `«${model.value}»` : ''
})

function pick(value: string) {
  model.value = value
  open.value = false
}

function reset() {
  model.value = ''
  from.value = ''
  to.value = ''
  open.value = false
}
</script>

<template>
  <span ref="root" class="filter" :class="{ right }">
    <button type="button" class="pill" :class="{ open, active }" :aria-expanded="open" @click="open = !open">
      <span v-if="active" class="dot" />
      <span class="prefix">{{ label }}:</span>
      <span class="value" :class="{ empty: !active }">{{ active ? summary : 'все' }}</span>
      <span v-if="active" class="clear-x" role="button" :aria-label="`Сбросить: ${label.toLowerCase()}`" @click.stop="reset">
        <UiIcon name="close" :size="11" />
      </span>
      <UiIcon v-else name="down" :size="14" class="caret" :class="{ flip: open }" />
    </button>

    <Transition name="menu">
      <div v-if="open" class="menu">
        <template v-if="options">
          <button type="button" class="option muted" @click="pick('')">все</button>
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            class="option"
            :class="{ active: option.value === model }"
            @click="pick(option.value)"
          >
            <span>{{ option.label }}</span>
            <span class="meta mono">{{ option.meta }}</span>
            <span class="check">{{ option.value === model ? '✓' : '' }}</span>
          </button>
        </template>

        <p v-else-if="blocked" class="blocked">{{ blocked }}</p>

        <!-- dates: two rows that open the usual calendar, nothing to type -->
        <template v-else-if="range === 'date'">
          <button type="button" class="option muted" @click="reset">все</button>
          <UiDatePicker v-model="from" bare prefix="С" placeholder="любая дата" class="range-row" />
          <UiDatePicker v-model="to" bare prefix="По" placeholder="любая дата" class="range-row" />
        </template>

        <template v-else-if="range">
          <button type="button" class="option muted" @click="reset">все</button>
          <label class="field-row">
            <span class="range-label">От</span>
            <UiInput
              v-model="from"
              :filter="moneyFilter"
              :suffix="unit"
              inputmode="decimal"
              placeholder="0"
              mono
              bare
              class="range-input"
              :aria-label="`${label}: от`"
              @keydown.enter="open = false"
            />
          </label>
          <label class="field-row">
            <span class="range-label">До</span>
            <UiInput
              v-model="to"
              :filter="moneyFilter"
              :suffix="unit"
              inputmode="decimal"
              placeholder="0"
              mono
              bare
              class="range-input"
              :aria-label="`${label}: до`"
              @keydown.enter="open = false"
            />
          </label>
        </template>

        <label v-else class="field-row">
          <UiIcon name="search" :size="14" class="search" />
          <UiInput v-model="model" bare class="text" placeholder="Содержит…" :aria-label="label" maxlength="100" @keydown.enter="open = false" />
        </label>
      </div>
    </Transition>
  </span>
</template>

<style scoped>
.filter {
  position: relative;
  display: inline-flex;
  min-width: 0;
}

/* a pill like UiDropdown: «Сумма: от 1 000 ₽». the value shows what's picked, the cross takes it off */
.pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px 0 14px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 9999px;
  font-size: 13px;
  color: var(--ink);
  white-space: nowrap;
  transition: border-color 0.2s ease, box-shadow 0.35s var(--ease-out);
}

.pill:hover:not(.open) {
  border-color: var(--ink-3);
}

.pill.open {
  border-color: var(--ink-2);
  box-shadow: 0 0 0 3px var(--subtle);
  animation: focus-ring 0.45s var(--ease-out);
}

.prefix {
  color: var(--ink-3);
}

.value {
  max-width: 220px;
  overflow: hidden;
  font-weight: 500;
  text-overflow: ellipsis;
}

.value.empty {
  font-weight: 400;
  color: var(--ink-3);
}

.caret {
  color: var(--ink-3);
  transition: transform 0.35s var(--ease-out);
}

.caret.flip {
  transform: rotate(180deg);
}

/* takes the filter off without opening the menu */
.clear-x {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  margin-right: -4px;
  padding: 0;
  background: none;
  border: 0;
  border-radius: 50%;
  color: var(--ink-3);
  transition: background-color 0.2s ease, color 0.2s ease;
}

.clear-x:hover {
  background: var(--subtle);
  color: var(--ink);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent);
  animation: dot-in 0.4s var(--ease-spring);
}

/* the filter just got set: the dot pops in */
@keyframes dot-in {
  from {
    transform: scale(0);
  }
}

.menu {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  z-index: 5;
  display: flex;
  flex-direction: column;
  min-width: 200px;
  width: max-content;
  padding: 4px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--shadow-pop);
  font-size: 13px;
  font-weight: 400;
  color: var(--ink);
}

.right .menu {
  left: auto;
  right: 0;
}

.option {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 34px;
  padding: 0 10px;
  background: none;
  border: 0;
  border-radius: 9999px;
  text-align: left;
  white-space: nowrap;
}

.option:hover {
  background: var(--subtle);
}

.option.active {
  background: var(--subtle);
  font-weight: 500;
}

.option.muted {
  color: var(--ink-3);
}

.meta {
  margin-left: auto;
  font-size: 12px;
  color: var(--ink-3);
}

.check {
  width: 12px;
  margin-left: auto;
  color: var(--accent-ink);
}

.meta + .check {
  margin-left: 0;
}

/* a field in the menu: no frame and no fill, just a row like the options */
.field-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-left: 10px;
}

.search {
  flex-shrink: 0;
  color: var(--ink-3);
}

.text {
  width: 220px;
}

.range-label {
  width: 22px;
  font-size: 12px;
  color: var(--ink-3);
}

.range-input {
  width: 170px;
}

.range-row {
  width: 230px;
}

/* why the filter can't be used right now */
.blocked {
  max-width: 240px;
  margin: 6px 10px;
  font-size: 13px;
  line-height: 18px;
  color: var(--ink-2);
}

/* phones */
@media (max-width: 1240px) {
  /* the menu is a sheet at the bottom edge: it never runs off the side of the screen */
  .menu,
  .right .menu {
    position: fixed;
    top: auto;
    right: 12px;
    bottom: calc(12px + env(safe-area-inset-bottom, 0px));
    left: 12px;
    z-index: 70;
    width: auto;
    min-width: 0;
    max-height: 70dvh;
    overflow-y: auto;
    padding: 6px;
    border-radius: 16px;
  }

  .option {
    height: 44px;
    padding: 0 14px;
  }

  .field-row {
    padding-left: 14px;
  }

  .text,
  .range-row {
    width: 100%;
  }

  .field-row .text,
  .range-input {
    flex: 1 1 auto;
    width: auto;
    min-width: 0;
  }

  .blocked {
    max-width: none;
  }
}
</style>
