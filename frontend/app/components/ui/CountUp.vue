<!-- a formatted amount like «6 000,00 ₽» that runs up (or down) to its new value
     instead of jumping. the text around the number stays as it is -->
<script setup lang="ts">
const props = defineProps<{ value: string }>()

// the number inside the text: «1 500,00» → 1500, two digits after the comma
function parse(text: string) {
  const match = text.match(/-?\d(?:[\d\s]*\d)?(?:,\d+)?/)
  if (!match) return undefined
  const raw = match[0]
  const decimals = raw.includes(',') ? raw.split(',')[1]!.length : 0
  const number = Number(raw.replace(/\s/g, '').replace(',', '.'))
  return { number, decimals, before: text.slice(0, match.index), after: text.slice(match.index! + raw.length) }
}

const format = (number: number, decimals: number) =>
  number.toLocaleString('ru-RU', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })

const shown = ref(props.value)
let frame = 0
// the number on screen right now, so a new value starts from there
let current = parse(props.value)?.number ?? 0

const reduced = () => document.documentElement.dataset.motion === 'reduced'
  || window.matchMedia('(prefers-reduced-motion: reduce)').matches

// the first number also runs up from zero, the later ones from the old value
onMounted(() => run(props.value, 0))
watch(() => props.value, value => run(value, current))

function run(text: string, from: number) {
  cancelAnimationFrame(frame)
  const target = parse(text)
  if (!target || reduced() || from === target.number) {
    shown.value = text
    current = target?.number ?? 0
    return
  }
  const start = performance.now()
  const duration = 900
  const tick = (now: number) => {
    const t = Math.min(1, (now - start) / duration)
    // fast start, slow landing, like --ease-out
    const eased = 1 - Math.pow(1 - t, 4)
    current = from + (target.number - from) * eased
    shown.value = t < 1 ? `${target.before}${format(current, target.decimals)}${target.after}` : text
    if (t < 1) frame = requestAnimationFrame(tick)
  }
  frame = requestAnimationFrame(tick)
}

onBeforeUnmount(() => cancelAnimationFrame(frame))
</script>

<template>
  <span class="count-up">{{ shown }}</span>
</template>

<style scoped>
/* digits keep the same width while running, so the text doesn't jitter */
.count-up {
  font-variant-numeric: tabular-nums;
}
</style>
