<!-- a code from a letter, one cell per digit. typing, pasting and the phone's
     «code from sms/mail» go into one hidden field laid over the cells -->
<script setup lang="ts">
const props = withDefaults(defineProps<{
  length?: number
  // being checked: a light runs over the cells
  busy?: boolean
  // the code was right: the cells go green one after another
  success?: boolean
  label?: string
  // ready to type as soon as it shows up
  autofocus?: boolean
}>(), { length: 6, busy: false, success: false, label: 'Код', autofocus: false })

const model = defineModel<string>({ default: '' })

const field = ref<HTMLInputElement>()
const focused = ref(false)
const cells = computed(() => Array.from({ length: props.length }, (_, i) => model.value[i] ?? ''))
// the cell the next digit goes into
const active = computed(() => Math.min(model.value.length, props.length - 1))

function onInput(e: Event) {
  const el = e.target as HTMLInputElement
  const digits = el.value.replace(/\D/g, '').slice(0, props.length)
  if (digits !== el.value) el.value = digits
  model.value = digits
}

// a wrong code: the row shakes and the cells flash red, then empty out for the next try
const shaking = ref(false)
function shake() {
  shaking.value = false
  // next frame, so the animation starts again even right after the previous one
  requestAnimationFrame(() => (shaking.value = true))
}

onMounted(() => props.autofocus && field.value?.focus())

defineExpose({ focus: () => field.value?.focus(), shake })
</script>

<template>
  <span
    class="code-input"
    :class="{ focused, shaking, busy, success }"
    :style="{ '--n': length }"
    @animationend.self="shaking = false"
  >
    <input
      ref="field"
      :value="model"
      class="field"
      inputmode="numeric"
      autocomplete="one-time-code"
      :maxlength="length"
      :aria-label="label"
      :disabled="busy || success"
      @input="onInput"
      @focus="focused = true"
      @blur="focused = false"
    >
    <span
      v-for="(digit, i) in cells"
      :key="i"
      class="cell mono"
      :class="{ filled: digit, current: focused && i === active && !digit }"
      :style="{ '--i': i }"
    >
      <!-- a new key for every digit, so each one pops in -->
      <span v-if="digit" :key="digit + i" class="digit">{{ digit }}</span>
      <span v-else-if="focused && i === active" class="caret" />
    </span>
  </span>
</template>

<style scoped>
.code-input {
  position: relative;
  display: inline-grid;
  grid-template-columns: repeat(var(--n), 36px);
  gap: 6px;
}

/* the real field: invisible, over all the cells, so a click anywhere focuses it */
.field {
  position: absolute;
  inset: 0;
  z-index: 1;
  width: 100%;
  opacity: 0;
  cursor: text;
}

.cell {
  position: relative;
  display: grid;
  place-items: center;
  height: 40px;
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 10px;
  font-size: 17px;
  font-weight: 500;
  transition: border-color 0.2s ease, box-shadow 0.35s var(--ease-out), transform 0.35s var(--ease-spring), background-color 0.3s ease;
}

.focused .cell {
  border-color: var(--ink-3);
}

/* the cell waiting for a digit: lifted a little, with the soft ring like other fields */
.cell.current {
  border-color: var(--ink-2);
  box-shadow: 0 0 0 3px var(--subtle);
  transform: translateY(-2px);
}

.cell.filled {
  border-color: var(--ink-2);
}

.digit {
  animation: digit-in 0.4s var(--ease-spring);
}

@keyframes digit-in {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.5);
  }
}

.caret {
  width: 1.5px;
  height: 18px;
  border-radius: 1px;
  background: var(--ink);
  animation: blink 1s steps(2, start) infinite;
}

@keyframes blink {
  to {
    visibility: hidden;
  }
}

/* checking: a light sweeps over the cells, one after another */
.busy .cell::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(100deg, transparent 20%, color-mix(in srgb, var(--ink) 12%, transparent) 50%, transparent 80%);
  animation: sweep 1.1s ease-in-out infinite;
  animation-delay: calc(var(--i) * 70ms);
}

@keyframes sweep {
  from {
    transform: translateX(-100%);
  }

  to {
    transform: translateX(100%);
  }
}

/* wrong: a short shake, the cells flash red */
.shaking {
  animation: shake 0.45s cubic-bezier(0.36, 0.07, 0.19, 0.97);
}

.shaking .cell {
  border-color: var(--red);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--red) 18%, transparent);
}

@keyframes shake {
  20%,
  60% {
    transform: translateX(-6px);
  }

  40%,
  80% {
    transform: translateX(6px);
  }
}

/* right: a green wave runs through the cells */
.success .cell {
  border-color: var(--green);
  color: var(--green);
  animation: wave 0.5s var(--ease-spring) backwards;
  animation-delay: calc(var(--i) * 60ms);
}

@keyframes wave {
  40% {
    transform: translateY(-5px) scale(1.06);
    background: color-mix(in srgb, var(--green) 14%, var(--surface));
  }
}
</style>
