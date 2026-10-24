<script setup lang="ts">
export interface DropdownOption {
  value: string
  label: string
  meta?: string
}

const props = withDefaults(defineProps<{
  options: DropdownOption[]
  placeholder: string
  // offer the placeholder as an option to pick "nothing" (e.g. «Без персоны»)
  clearable?: boolean
  // muted text before the value, e.g. «Ответственный:»
  prefix?: string
  // 34px tall, to sit in a row of filters
  compact?: boolean
}>(), { clearable: true, prefix: '', compact: false })

// '' means nothing picked
const model = defineModel<string>({ default: '' })
const open = ref(false)
const root = ref<HTMLElement>()
useClickOutside(root, () => { open.value = false }, open)
// set by a surrounding UiField so its label points here
const fieldId = inject<string | undefined>('fieldId', undefined)

const current = computed(() => props.options.find(option => option.value === model.value))

function pick(value: string) {
  model.value = value
  open.value = false
}
</script>

<template>
  <div ref="root" class="dropdown">
    <button :id="fieldId" type="button" class="control" :class="{ open, compact }" @click="open = !open">
      <span class="text">
        <span v-if="prefix" class="prefix">{{ prefix }}</span>
        <span :class="{ muted: !current && !prefix, value: prefix }">{{ current?.label ?? placeholder }}</span>
      </span>
      <span class="muted">{{ open ? '▴' : '▾' }}</span>
    </button>

    <Transition name="menu">
      <div v-if="open" class="menu">
        <button v-if="clearable" type="button" class="option muted" @click="pick('')">{{ placeholder }}</button>
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
        <div v-if="$slots.footer" class="footer">
          <slot name="footer" />
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.dropdown {
  position: relative;
}

.control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  height: 40px;
  padding: 0 12px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 6px;
  text-align: left;
}

/* one line only, a long value ends with … instead of wrapping */
.text {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.control.compact {
  height: 34px;
  font-size: 13px;
}

.prefix {
  margin-right: 8px;
  color: var(--ink-3);
}

.value {
  font-weight: 500;
}

.control.open {
  border-color: var(--accent-ink);
}

/* at least as wide as the control, wider if an option needs it */
.menu {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  min-width: 100%;
  width: max-content;
  z-index: 1;
  padding: 4px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
}

.option {
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 34px;
  padding: 0 10px;
  background: none;
  border: 0;
  border-radius: 5px;
  text-align: left;
}

.option.active {
  background: var(--subtle);
  font-weight: 500;
}

.meta {
  margin-left: auto;
  font-size: 12px;
  color: var(--ink-3);
}

.check {
  width: 12px;
  color: var(--accent-ink);
}

.footer {
  margin: 4px -4px -4px;
  padding: 12px 14px;
  border-top: 1px solid var(--line);
}
</style>
