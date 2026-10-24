<script setup lang="ts">
// question bar in the page header. asking opens the chat under it; a click outside closes the chat
// and the bar turns into a button that brings the same chat back
const chat = useAssistant()
const { messages, open } = chat

const question = ref('')
const root = ref<HTMLElement>()
const input = ref<HTMLInputElement>()
const sizer = ref<HTMLElement>()
const scroller = ref<HTMLElement>()

onMounted(chat.loadStatus)

// keep the newest text in view, unless the person scrolled up to read
let stick = true
let lastTop = 0

useClickOutside(root, () => (open.value = false), open)

// the earlier dialogue comes back from the server the first time the chat opens
watch(open, (value) => {
  if (value) chat.loadHistory()
})

function send() {
  if (chat.busy.value) return
  stick = true
  chat.ask(question.value)
  question.value = ''
}

async function reopen() {
  open.value = true
  await nextTick()
  input.value?.focus()
}

function newChat() {
  chat.clear()
  input.value?.focus()
}

const showChat = computed(() => open.value && messages.value.length > 0)
const placeholder = computed(() => (showChat.value
  ? 'Спросить ещё…'
  : `Спросить: «${(chat.examples.value[0] ?? 'Что просрочено?').toLowerCase()}»`))

// the window is as wide as its text needs: from the bar's width up to about a quarter of the screen,
// and as tall as fits below the bar
const BAR_WIDTH = 300
const width = ref(BAR_WIDTH)
const maxWidth = ref(480)
const maxHeight = ref(400)

function measure() {
  maxWidth.value = Math.max(BAR_WIDTH + 40, Math.round(window.innerWidth / 4))
  const bottom = root.value?.getBoundingClientRect().bottom ?? 0
  maxHeight.value = Math.max(160, window.innerHeight - bottom - 22)
  if (sizer.value) {
    // 34 = side paddings + room for the scrollbar
    width.value = Math.min(maxWidth.value, Math.max(BAR_WIDTH, sizer.value.offsetWidth + 34))
  }
  if (stick && scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
  updateThumb()
}

// our own scrollbar: system ones hide themselves on some systems until you scroll
const TRACK_GAP = 12
const thumb = ref({ top: 0, height: 0 })
const overflows = ref(false)

function updateThumb() {
  const el = scroller.value
  if (!el) return
  overflows.value = el.scrollHeight > el.clientHeight + 1
  const track = el.clientHeight - TRACK_GAP
  const height = Math.max(24, track * el.clientHeight / el.scrollHeight)
  const top = (track - height) * el.scrollTop / Math.max(1, el.scrollHeight - el.clientHeight)
  thumb.value = { top, height }
}

function dragThumb(e: PointerEvent) {
  const el = scroller.value!
  const startY = e.clientY
  const startTop = el.scrollTop
  // how many pixels of text one pixel of thumb movement is
  const ratio = (el.scrollHeight - el.clientHeight) / (el.clientHeight - TRACK_GAP - thumb.value.height)
  const move = (ev: PointerEvent) => {
    el.scrollTop = startTop + (ev.clientY - startY) * ratio
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', () => window.removeEventListener('pointermove', move), { once: true })
}

// a click on the empty track scrolls one window up or down
function pageTo(e: PointerEvent) {
  const el = scroller.value!
  const clickY = e.clientY - (e.currentTarget as HTMLElement).getBoundingClientRect().top
  el.scrollTop += clickY < thumb.value.top ? -el.clientHeight : el.clientHeight
}

function onScroll() {
  const el = scroller.value
  // the last scroll event can come after the chat has closed
  if (!el) return
  // new text also moves the bottom away, so only scrolling up counts as "stop following"
  if (el.scrollHeight - el.scrollTop - el.clientHeight < 40) stick = true
  else if (el.scrollTop < lastTop) stick = false
  lastTop = el.scrollTop
  updateThumb()
}

let observer: ResizeObserver | undefined
watch(sizer, (el) => {
  observer?.disconnect()
  if (!el) return
  observer = new ResizeObserver(measure)
  observer.observe(el)
  measure()
})

onMounted(() => {
  window.addEventListener('resize', measure)
  window.addEventListener('scroll', measure, { passive: true })
})
onBeforeUnmount(() => {
  observer?.disconnect()
  window.removeEventListener('resize', measure)
  window.removeEventListener('scroll', measure)
})
</script>

<template>
  <div v-if="chat.available.value" ref="root" class="wrap">
    <button v-if="!open && messages.length" type="button" class="ask resume" @click="reopen">
      <span class="dot" />Чат с ассистентом
      <span class="count mono">{{ messages.filter(message => message.from === 'me').length }}</span>
    </button>
    <form v-else class="ask" @submit.prevent="send">
      <UiIcon name="search" :size="15" class="search" />
      <input
        ref="input"
        v-model="question"
        class="input"
        :placeholder="placeholder"
        aria-label="Вопрос ассистенту"
        maxlength="500"
        @focus="open = true"
      >
    </form>

    <Transition name="menu">
      <div
        v-if="showChat"
        class="chat card"
        :style="{ width: `${width}px`, maxHeight: `${maxHeight}px` }"
        role="log"
        aria-live="polite"
      >
        <div class="head">
          <span class="kicker">Ассистент<template v-if="chat.memory.value"> · {{ chat.memory.value }}</template></span>
          <button type="button" class="new" @click="newChat">Новый чат</button>
        </div>

        <div class="body">
          <div ref="scroller" class="messages" tabindex="0" @scroll="onScroll">
            <div ref="sizer" class="sizer" :style="{ maxWidth: `${maxWidth - 34}px` }">
              <template v-for="message in messages" :key="message.id">
                <p v-if="message.from === 'me'" class="mine" :class="{ forgotten: message.forgotten }">{{ message.text }}</p>
                <p v-else-if="message.pending" class="pending"><AppMascot pose="jump" :size="30" />{{ message.step }}…</p>
                <template v-else>
                  <p class="answer" :class="{ error: message.error, typing: message.typing, forgotten: message.forgotten }">{{ message.text }}</p>
                  <div v-if="message.followups?.length && !message.typing" class="followups">
                    <button
                      v-for="followup in message.followups"
                      :key="followup"
                      type="button"
                      class="followup"
                      :disabled="chat.busy.value"
                      @click="chat.ask(followup)"
                    >
                      {{ followup }}
                    </button>
                  </div>
                </template>
              </template>
            </div>
          </div>
          <div v-if="overflows" class="track" aria-hidden="true" @pointerdown.self="pageTo">
            <div
              class="thumb"
              :style="{ top: `${thumb.top}px`, height: `${thumb.height}px` }"
              @pointerdown.prevent="dragThumb"
            />
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.wrap {
  position: relative;
}

.ask {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 300px;
  height: 34px;
  padding: 0 14px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 9999px;
  font-size: 13px;
  color: var(--ink-2);
}

.ask {
  transition: border-color 0.2s ease, box-shadow 0.3s var(--ease-out);
}

.ask:focus-within {
  border-color: var(--ink-2);
  box-shadow: 0 0 0 3px var(--subtle);
  animation: focus-ring 0.45s var(--ease-out);
}

.search {
  color: var(--ink-3);
  transition: color 0.2s ease, transform 0.45s var(--ease-spring);
}

.ask:focus-within .search {
  color: var(--ink);
  transform: scale(1.1);
}

.resume:hover {
  color: var(--ink);
  border-color: var(--line-strong);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-ink);
}

.count {
  margin-left: auto;
  font-size: 11px;
  color: var(--ink-3);
}

.input {
  flex: 1;
  border: 0;
  outline: none;
  background: none;
  font-size: 13px;
}

.input::placeholder {
  color: var(--ink-3);
}

/* opens under the bar, grows to the left as the answer gets longer */
.chat {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 20;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: var(--shadow-pop);
  transition: width 0.2s ease;
}

.head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px 8px;
  border-bottom: 1px solid var(--line);
}

.new {
  margin-left: auto;
  padding: 0;
  background: none;
  border: 0;
  font-size: 12px;
  color: var(--ink-3);
}

.new:hover {
  color: var(--ink);
}

.body {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
}

/* the right padding keeps room for the scrollbar, so text doesn't jump when it shows up */
.messages {
  flex: 1;
  padding: 12px 18px 14px 14px;
  overflow-y: auto;
  outline: none;
  scrollbar-width: none;
}

.messages::-webkit-scrollbar {
  display: none;
}

.track {
  position: absolute;
  top: 6px;
  right: 5px;
  bottom: 6px;
  width: 6px;
  border-radius: 9999px;
  background: var(--subtle);
}

.thumb {
  position: absolute;
  left: 0;
  width: 6px;
  border-radius: 9999px;
  background: var(--line-strong);
  cursor: grab;
}

.thumb:hover,
.thumb:active {
  background: var(--ink-3);
}

.sizer {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: max-content;
}

/* earlier turns the assistant no longer keeps in mind: still readable, just quieter */
.forgotten {
  opacity: 0.55;
}

.mine {
  align-self: flex-end;
  max-width: 85%;
  padding: 6px 10px;
  background: var(--muted-bg);
  border-radius: 16px 16px 4px 16px;
  font-size: 13px;
}

.pending {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  font-size: 13px;
  color: var(--ink-3);
}

.answer {
  font-size: 13px;
  line-height: 19px;
  white-space: pre-line;
}

.answer.error {
  color: var(--red);
}

.answer.typing::after {
  content: '▍';
  color: var(--accent-ink);
}

.followups {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.followup {
  padding: 3px 9px;
  background: none;
  border: 1px solid var(--line-strong);
  border-radius: 9999px;
  font-size: 12px;
  color: var(--ink-2);
}

.followup:hover:not(:disabled) {
  color: var(--ink);
  border-color: var(--accent-ink);
}
</style>
