<script setup lang="ts">
import type { Currency } from '~/utils/periods'

// the "receipt" on the right side of the login screen, pure decoration.
// on open it prints line by line, the totals count up and the stamp lands, then the caption types itself.
// after that a status changes now and then, so the slip looks like it's working.
// with less motion on (settings or the system) it's all shown at once and stays still
type Tone = 'red' | 'soft' | 'green' | 'muted'
const start = [
  { date: '18.10', title: 'Аренда офиса', amount: '120 000,00 ₽', note: 'ПРОСРОЧЕНО · 6 ДН.', tone: 'red' as Tone },
  { date: '25.10', title: 'Сервер VPS', amount: '49,90 $', note: 'ЗАВТРА', tone: 'soft' as Tone },
  { date: '26.10', title: 'Дизайн-сервис', amount: '15,00 €', note: 'ОПЛАЧЕНО 20.10', tone: 'green' as Tone },
  { date: '30.10', title: 'Почта Workspace', amount: '36,00 €', note: 'ЧЕРЕЗ 6 ДН.', tone: 'muted' as Tone },
  { date: '31.10', title: 'Домен', amount: '1 990,00 ₽', note: 'ЧЕРЕЗ 7 ДН.', tone: 'muted' as Tone },
]
const lines = ref(start.map(line => ({ ...line })))

const totals: { code: Currency, value: number }[] = [
  { code: 'RUB', value: 123490 },
  { code: 'USD', value: 61.9 },
  { code: 'EUR', value: 51 },
]
const caption = 'Телеграм уведомления за три дня до срока'

const { reduceMotion } = usePrefs()
// print: the slip comes out of the printer, the lines follow (css, see .printing)
const printing = ref(false)
const counted = ref(0)
const typed = ref(caption.length)
const typing = ref(false)
// a new key replays the stamp landing
const stampKey = ref(0)
// every time the stamp lands it's at a new angle, up to 25° either way, like stamped by hand
const randomTilt = () => Math.round((Math.random() * 2 - 1) * 25)
const tilt = ref(randomTilt())
watch(stampKey, () => { tilt.value = randomTilt() })

const timers: ReturnType<typeof setTimeout>[] = []
const later = (ms: number, fn: () => void) => timers.push(setTimeout(fn, ms))
let frame = 0

// totals go from 0 to their value, fast at first and slowing down at the end
function countUp(ms: number) {
  const from = performance.now()
  const step = (now: number) => {
    const t = Math.min(1, (now - from) / ms)
    counted.value = 1 - (1 - t) ** 3
    if (t < 1) frame = requestAnimationFrame(step)
  }
  frame = requestAnimationFrame(step)
}

function typeCaption() {
  typed.value = 0
  typing.value = true
  const next = () => {
    typed.value++
    if (typed.value < caption.length) later(38, next)
    else later(1600, () => (typing.value = false))
  }
  later(38, next)
}

// the status text turns into the new one through random letters, like a split-flap board
const letters = 'АБВГДЕЖЗИКЛМНОПРСТУФХЦЧШЭЮЯ0123456789'
function scramble(index: number, note: string, tone: Tone) {
  const line = lines.value[index]!
  const steps = 14
  let n = 0
  const tick = () => {
    n++
    const keep = Math.floor((note.length * n) / steps)
    const noise = Array.from({ length: Math.max(0, note.length - keep) }, () => letters[Math.floor(Math.random() * letters.length)]).join('')
    line.note = note.slice(0, keep) + noise
    if (n === Math.ceil(steps / 2)) line.tone = tone
    if (n < steps) later(36, tick)
    else line.note = note
  }
  tick()
}

// what happens on the slip, one change every few seconds, then everything goes back and it starts again
const changes: { index: number, note: string, tone: Tone }[] = [
  { index: 1, note: 'ОПЛАЧЕНО 24.10', tone: 'green' },
  { index: 0, note: 'ОПЛАЧЕНО 24.10', tone: 'green' },
  { index: 3, note: 'ОПЛАЧЕНО 24.10', tone: 'green' },
]
let step = 0
function live() {
  if (step < changes.length) {
    const change = changes[step]!
    scramble(change.index, change.note, change.tone)
    later(600, () => stampKey.value++)
    step++
  }
  else {
    start.forEach((line, i) => {
      if (lines.value[i]!.note !== line.note) scramble(i, line.note, line.tone)
    })
    step = 0
  }
  later(3800, live)
}

onMounted(() => {
  if (reduceMotion.value) {
    counted.value = 1
    return
  }
  printing.value = true
  typed.value = 0
  later(900, () => countUp(900))
  later(1900, () => stampKey.value++)
  later(2400, typeCaption)
  later(6500, live)
})

onBeforeUnmount(() => {
  timers.forEach(clearTimeout)
  cancelAnimationFrame(frame)
})
</script>

<template>
  <div class="receipt-wrap">
    <div class="receipt mono" :class="{ printing }">
      <p class="head"><span class="strong">НАПОМИНАНИЯ</span><span class="muted">ОКТ 2026</span></p>
      <p class="dashes">- - - - - - - - - - - - - - - - - - - - - - - -</p>

      <div v-for="(line, i) in lines" :key="line.date" class="line" :style="{ '--i': i }">
        <span class="date soft">{{ line.date }}</span>
        <span>
          <span class="title">{{ line.title }}</span>
          <span class="note" :class="line.tone">{{ line.note }}</span>
        </span>
        <span class="amount">{{ line.amount }}</span>
      </div>

      <p class="dashes">- - - - - - - - - - - - - - - - - - - - - - - -</p>
      <p class="kicker">Итого за месяц</p>
      <UiLeaderRow v-for="total in totals" :key="total.code">
        <template #label><span class="soft">{{ total.code }}</span></template>
        <span class="strong">{{ formatMoney(total.value * counted, total.code) }}</span>
      </UiLeaderRow>

      <div :key="stampKey" class="stamp-spot" :class="{ hidden: printing && !stampKey }">
        <UiStamp color="accent" :tilt="tilt" large class="stamp">ПРОВЕРЕНО 24.10</UiStamp>
      </div>
    </div>

    <p class="caption italic">
      <span>{{ caption.slice(0, typed) }}</span><span v-if="typing" class="caret" aria-hidden="true" />
      <!-- keeps the line's place while it types, screen readers get the whole text -->
      <span class="ghost" aria-hidden="true">{{ caption.slice(typed) }}</span>
    </p>
  </div>
</template>

<style scoped>
.receipt-wrap {
  width: 380px;
}

.receipt {
  /* swap theme colours for paper ones, everything inside picks them up */
  --surface: var(--paper);
  --ink: var(--paper-ink);
  --ink-2: var(--paper-ink-2);
  --ink-3: var(--paper-ink-3);
  --line-strong: var(--paper-line);
  --accent: var(--paper-accent);
  --accent-ink: var(--paper-accent);
  --red: var(--paper-red);
  --green: var(--paper-green);

  position: relative;
  color: var(--ink);
  /* extra room at the bottom so the stamp fits on the paper */
  padding: 28px 28px 124px;
  background: var(--surface);
  font-size: 13px;
}

/* torn paper edge at the bottom */
.receipt::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -7px;
  height: 7px;
  background:
    linear-gradient(135deg, var(--surface) 50%, transparent 50%) 0 0 / 10px 7px repeat-x,
    linear-gradient(225deg, var(--surface) 50%, transparent 50%) 0 0 / 10px 7px repeat-x;
}

.head {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  letter-spacing: 0.08em;
}

.strong {
  font-weight: 500;
}

.dashes {
  margin: 8px 0 14px;
  overflow: hidden;
  font-size: 11px;
  white-space: nowrap;
  color: var(--line-strong);
}

.line {
  display: grid;
  grid-template-columns: 62px 1fr auto;
  column-gap: 16px;
  height: 52px;
}

.title {
  display: block;
  font-family: var(--sans);
  font-size: 14px;
  font-weight: 500;
}

.note {
  font-size: 10px;
  letter-spacing: 0.04em;
}

.note.red { color: var(--red); }
.note.green { color: var(--green); }
.note.soft { color: var(--ink-2); }
.note.muted { color: var(--ink-3); }

/* a printed slip: its labels stay in mono caps */
.kicker {
  margin-bottom: 9px;
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.receipt :deep(.row) {
  margin-bottom: 5px;
}

.stamp-spot {
  position: absolute;
  left: 0;
  right: 0;
  /* higher than before: a steep angle needs room under it too */
  bottom: 52px;
  text-align: center;
}

.stamp {
  padding: 14px 22px;
  font-size: 15px;
  border-width: 2px;
  transform-origin: center;
}

.caption {
  margin-top: 26px;
  font-size: 15px;
  line-height: 21px;
  color: var(--ink-2);
}

/* ---- motion, only when it's allowed (onMounted in the script) ---- */

/* the slip comes out from under the top edge, like from a till printer */
.printing {
  animation: print 0.9s var(--ease-out) both;
}

@keyframes print {
  from {
    clip-path: inset(0 0 100% 0);
    transform: translateY(-12px);
  }

  /* a bit below the bottom, so the torn edge shows */
  to {
    clip-path: inset(0 0 -12px 0);
    transform: none;
  }
}

/* the lines show up one after another as the paper comes out */
.printing .line {
  animation: print-line 0.4s var(--ease-out) backwards;
  animation-delay: calc(150ms + var(--i) * 110ms);
}

@keyframes print-line {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }
}

.stamp-spot.hidden {
  visibility: hidden;
}

/* the stamp lands: from big and see-through down onto the paper, with a small bounce */
.printing .stamp-spot:not(.hidden) {
  animation: stamp-land 0.45s var(--ease-spring) both;
}

@keyframes stamp-land {
  from {
    opacity: 0;
    transform: scale(1.8);
  }

  60% {
    opacity: 1;
  }
}

.caret {
  display: inline-block;
  width: 1px;
  height: 1.05em;
  margin-left: 1px;
  vertical-align: -2px;
  background: currentColor;
  animation: blink 1s steps(1) infinite;
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

/* the rest of the caption, invisible, holds its place while it types */
.ghost {
  visibility: hidden;
}
</style>
