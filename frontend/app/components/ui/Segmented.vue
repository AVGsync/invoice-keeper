<script setup lang="ts">
export interface SegmentedOption {
  value: string
  label: string
  count?: number
  danger?: boolean
}

const props = withDefaults(defineProps<{
  options: SegmentedOption[]
  // accent = blue active segment, like the currency picker
  tone?: 'plain' | 'accent'
  mono?: boolean
}>(), { tone: 'plain' })

const model = defineModel<string>()

// one highlight ("thumb") slides from the old choice to the new one
const root = ref<HTMLElement>()
const thumb = ref({ left: 0, width: 0 })
// false until the thumb has been placed once, so it doesn't fly in from the left on load
const measured = ref(false)
const sliding = ref(false)

function measure() {
  const active = root.value?.querySelector<HTMLElement>('.item.active')
  if (!active) return
  thumb.value = { left: active.offsetLeft, width: active.offsetWidth }
  if (!measured.value) {
    measured.value = true
    // turn the slide on only after the first placement has been painted
    requestAnimationFrame(() => { sliding.value = true })
  }
}

watch(() => [model.value, props.options], () => nextTick(measure), { deep: true })

// counts and fonts change the widths, so measure again when the control resizes
let observer: ResizeObserver | undefined
onMounted(() => {
  measure()
  observer = new ResizeObserver(measure)
  observer.observe(root.value!)
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div ref="root" class="seg" :class="[tone, { measured, sliding }]">
    <span
      v-if="measured"
      class="thumb"
      :style="{ width: `${thumb.width}px`, transform: `translateX(${thumb.left}px)` }"
      aria-hidden="true"
    />
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="item"
      :class="{ active: model === option.value, mono }"
      :aria-pressed="model === option.value"
      @click="model = option.value"
    >
      {{ option.label }}
      <span v-if="option.count !== undefined" class="count mono" :class="{ danger: option.danger }">
        {{ option.count }}
      </span>
    </button>
  </div>
</template>

<style scoped>
.seg {
  position: relative;
  display: inline-flex;
  padding: 3px;
  background: var(--subtle);
  border: 1px solid var(--line);
  border-radius: 9999px;
}

.thumb {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 0;
  border-radius: 9999px;
}

.sliding .thumb {
  transition: transform 0.22s ease, width 0.22s ease;
}

.plain .thumb {
  background: var(--surface);
  border: 1px solid var(--line);
  box-shadow: var(--shadow-button);
}

.accent .thumb {
  background: var(--accent);
}

.item {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-width: 59px;
  height: 24px;
  padding: 0 12px;
  background: none;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 13px;
  color: var(--ink-2);
  transition: color 0.22s ease;
}

.item.mono {
  font-size: 12px;
  font-weight: 500;
}

.plain .active {
  color: var(--ink);
  font-weight: 500;
}

.accent .active {
  color: #fff;
}

/* before the thumb is placed (first render) the active item paints its own background */
.plain:not(.measured) .active {
  background: var(--surface);
  border-color: var(--line);
}

.accent:not(.measured) .active {
  background: var(--accent);
}

/* the accent version fills its column instead of growing past it */
.seg.accent {
  display: flex;
}

.accent .item {
  min-width: 0;
  height: 32px;
  flex: 1;
}

.count {
  font-size: 12px;
  font-weight: 400;
  color: var(--ink-3);
}

.count.danger {
  color: var(--red);
}

/* phones */
@media (max-width: 1240px) {
  .item {
    flex-shrink: 0;
    white-space: nowrap;
  }
}
</style>
