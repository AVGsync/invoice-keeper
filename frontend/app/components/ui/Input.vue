<script setup lang="ts">
import eye from '~/assets/icons/eye.svg'
import eyeOff from '~/assets/icons/eye-off.svg'

const props = withDefaults(defineProps<{
  mono?: boolean
  suffix?: string
  type?: string
  // cleans the text while typing, e.g. moneyFilter (utils/inputFilters.ts)
  filter?: (text: string) => string
  // tidies the text when the field loses focus, e.g. moneyFormat
  format?: (text: string) => string
}>(), { type: 'text', suffix: '', filter: undefined, format: undefined })

// autocomplete, placeholder etc. go to the <input>, not the wrapper
defineOptions({ inheritAttrs: false })

const model = defineModel<string>()
// set by a surrounding UiField so its label points here
const fieldId = inject<string | undefined>('fieldId', undefined)

// password fields get an eye button to show what was typed
const isPassword = computed(() => props.type === 'password')
const visible = ref(false)
const inputType = computed(() => (isPassword.value && visible.value ? 'text' : props.type))

function onInput(e: Event) {
  const el = e.target as HTMLInputElement
  const text = props.filter ? props.filter(el.value) : el.value
  // show the cleaned text right away, not only after the next render
  if (text !== el.value) el.value = text
  model.value = text
}

function onBlur() {
  if (props.format && model.value) model.value = props.format(model.value)
}
</script>

<template>
  <span class="wrap">
    <input
      :id="fieldId"
      :value="model"
      v-bind="$attrs"
      class="input"
      :class="{ mono, 'has-eye': isPassword }"
      :type="inputType"
      @input="onInput"
      @blur="onBlur"
    >
    <button
      v-if="isPassword"
      type="button"
      class="eye"
      :aria-label="visible ? 'Скрыть пароль' : 'Показать пароль'"
      @click="visible = !visible"
    >
      <img :src="visible ? eyeOff : eye" width="16" height="16" alt="">
    </button>
    <span v-else-if="suffix" class="suffix">{{ suffix }}</span>
  </span>
</template>

<style scoped>
.wrap {
  position: relative;
  display: block;
}

.input {
  width: 100%;
  height: 40px;
  padding: 0 16px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 9999px;
  font-size: 14px;
  outline: none;
}

.input.has-eye {
  padding-right: 40px;
}

.input:focus {
  border-color: var(--ink-2);
  box-shadow: 0 0 0 3px var(--subtle);
}

.input::placeholder {
  color: var(--ink-3);
}

.suffix {
  position: absolute;
  right: 16px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--ink-3);
  pointer-events: none;
}

.eye {
  position: absolute;
  right: 10px;
  top: 50%;
  display: flex;
  padding: 6px;
  background: none;
  border: 0;
  transform: translateY(-50%);
}
</style>
