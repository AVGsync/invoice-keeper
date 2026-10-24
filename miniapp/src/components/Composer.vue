<script setup lang="ts">
import { computed, ref, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { shouldSubmit } from '../lib/composer'
import { ui } from '../stores/ui'

const props = withDefaults(
  defineProps<{
    modelValue: string
    busy: boolean
    disabled?: boolean
    maxlength?: number
  }>(),
  { disabled: false, maxlength: 500 },
)

const emit = defineEmits<{
  'update:modelValue': [string]
  submit: []
  stop: []
  focus: []
  blur: []
}>()

const ta = ref<HTMLTextAreaElement | null>(null)
const formEl = ref<HTMLFormElement | null>(null)

const len = computed(() => props.modelValue.length)
const canSend = computed(() => {
  const n = props.modelValue.trim().length
  return n >= 1 && n <= props.maxlength && !props.disabled && !props.busy
})

function onInput(e: Event): void {
  emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
}

// Авто-рост: 1 → 5 строк, дальше внутренний скролл.
function autosize(): void {
  const el = ta.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 5 * 22 + 20) + 'px'
}

watch(
  () => props.modelValue,
  () => void nextTick(autosize),
)

function onKeydown(e: KeyboardEvent): void {
  if (shouldSubmit(e)) {
    e.preventDefault()
    if (canSend.value) emit('submit')
  }
}

function onButton(): void {
  if (props.busy) emit('stop')
  else if (canSend.value) emit('submit')
}

function onFocus(): void {
  ui.composerFocused = true
  emit('focus')
}
function onBlur(): void {
  ui.composerFocused = false
  emit('blur')
}

// Высота композера → --composer-h на ближайшем .ask (под фид кладём отступ).
let ro: ResizeObserver | null = null
function publishHeight(): void {
  const form = formEl.value
  if (!form) return
  const host = form.closest('.ask') as HTMLElement | null
  if (host) host.style.setProperty('--composer-h', form.offsetHeight + 'px')
}

onMounted(() => {
  autosize()
  publishHeight()
  if (typeof ResizeObserver !== 'undefined' && formEl.value) {
    ro = new ResizeObserver(() => publishHeight())
    ro.observe(formEl.value)
  }
})
onBeforeUnmount(() => {
  ro?.disconnect()
  ui.composerFocused = false
})

defineExpose({ focus: () => ta.value?.focus() })
</script>

<template>
  <form
    ref="formEl"
    class="composer"
    :class="{ 'composer--raised': ui.composerFocused }"
    role="search"
    @submit.prevent
  >
    <textarea
      ref="ta"
      rows="1"
      :maxlength="maxlength"
      :value="modelValue"
      enterkeyhint="send"
      aria-label="Вопрос ассистенту"
      placeholder="Спросите про платежи…"
      autocomplete="off"
      @input="onInput"
      @keydown="onKeydown"
      @focus="onFocus"
      @blur="onBlur"
    ></textarea>
    <span v-if="len > 400" class="counter" aria-live="polite">{{ len }}/{{ maxlength }}</span>
    <button
      type="button"
      class="send"
      :class="{ busy }"
      :aria-label="busy ? 'Остановить' : 'Отправить'"
      :disabled="!busy && !canSend"
      @click="onButton"
    >
      <svg v-if="!busy" class="arrow" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <path
          d="M12 19V5M5.5 11.5 12 5l6.5 6.5"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
      <span v-else class="stop" aria-hidden="true"></span>
    </button>
  </form>
</template>

<style scoped>
.composer {
  position: fixed;
  left: var(--tabbar-gap);
  right: var(--tabbar-gap);
  max-width: 520px;
  margin: 0 auto;
  z-index: 49;
  bottom: calc(var(--tabbar-h) + var(--tabbar-gap) * 2 + var(--safe-bottom));
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 6px 6px 6px 16px;
  border-radius: 24px;
  background: var(--dock-bg);
  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);
  border: 1px solid var(--line-strong);
  box-shadow: var(--dock-shadow);
  transition: bottom 0.4s var(--ease-drawer);
}
/* фокус поднимает композер над клавиатурой (панель вкладок при этом уезжает) */
.composer--raised {
  bottom: calc(8px + var(--safe-bottom));
}

.composer textarea {
  flex: 1;
  min-width: 0;
  border: 0;
  background: transparent;
  border-radius: 0;
  padding: 7px 0;
  min-height: 36px;
  font-size: 16px; /* иначе iOS зумит поле */
  line-height: 22px;
  color: var(--bone-text);
}
.composer textarea:focus {
  outline: none;
  background: transparent;
  border: 0;
}

.counter {
  align-self: center;
  font-size: var(--fs-micro);
  font-variant-numeric: tabular-nums;
  color: var(--muted);
  padding-bottom: 8px;
}

.send {
  position: relative;
  flex: 0 0 36px;
  width: 36px;
  height: 36px;
  border: 0;
  border-radius: 50%;
  background: var(--fill);
  color: var(--on-fill);
  display: flex;
  align-items: center;
  justify-content: center;
  transition:
    background-color var(--dur-press) var(--ease-press),
    transform var(--dur-press) var(--ease-press);
}
/* зона касания 44×44 */
.send::after {
  content: '';
  position: absolute;
  inset: -4px;
  border-radius: 50%;
}
.send:active:not(:disabled) {
  transform: scale(0.92);
}
.send:disabled {
  background: var(--graphite-2);
  color: var(--muted);
  opacity: 1;
}
.arrow {
  width: 20px;
  height: 20px;
}
.stop {
  width: 10px;
  height: 10px;
  background: currentColor;
  border-radius: 2px;
}
/* кольцо-спиннер вокруг кнопки во время стрима */
.send.busy::before {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 50%;
  border: 2px solid transparent;
  border-top-color: var(--accent);
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .send.busy::before {
    animation: none;
  }
  .composer {
    transition: none;
  }
}
</style>
