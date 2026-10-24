<!-- 12 months of one year to pick from, arrows change the year. used by UiMonthPicker and UiDatePicker -->
<script setup lang="ts">
type Month = { year: number, month: number }

const props = defineProps<{
  year: number
  selected?: Month
  // e.g. the month of "today", gets an outline
  current?: Month
}>()

const emit = defineEmits<{ pick: [year: number, month: number] }>()

const shownYear = ref(props.year)
watch(() => props.year, (year) => { shownYear.value = year })

// which way the year went, so the months slide in from that side
const way = ref<'next' | 'prev'>('next')
function stepYear(by: number) {
  way.value = by > 0 ? 'next' : 'prev'
  shownYear.value += by
}

const short = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек']

const is = (month: Month | undefined, index: number) => month?.year === shownYear.value && month.month === index
</script>

<template>
  <div class="month-grid">
    <div class="head">
      <button type="button" class="arrow" aria-label="Предыдущий год" @click="stepYear(-1)"><UiIcon name="left" :size="15" /></button>
      <span class="year mono">{{ shownYear }}</span>
      <button type="button" class="arrow" aria-label="Следующий год" @click="stepYear(1)"><UiIcon name="right" :size="15" /></button>
    </div>

    <Transition :name="`slide-${way}`" mode="out-in">
      <div :key="shownYear" class="grid">
        <button
          v-for="(name, index) in short"
          :key="name"
          type="button"
          class="month"
          :class="{ selected: is(selected, index), current: is(current, index) }"
          :style="{ '--i': index }"
          @click="emit('pick', shownYear, index)"
        >
          {{ name }}
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.year {
  font-size: 13px;
  font-weight: 500;
}

.arrow:hover {
  background: var(--subtle);
  color: var(--ink);
}

.arrow {
  display: inline-grid;
  place-items: center;
  width: 28px;
  height: 28px;
  background: none;
  border: 0;
  border-radius: 9999px;
  font-size: 15px;
  color: var(--ink-2);
}

.grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 4px;
}

/* the months pop in one after another, row by row */
.month {
  animation: pop 0.4s var(--ease-spring) backwards;
  animation-delay: calc(var(--i) * 18ms);
  height: 36px;
  background: none;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 13px;
}

.month:hover {
  background: var(--subtle);
}

.month.current {
  border-color: var(--accent);
}

.month.selected {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

@keyframes pop {
  from {
    opacity: 0;
    transform: scale(0.7);
  }
}
</style>
