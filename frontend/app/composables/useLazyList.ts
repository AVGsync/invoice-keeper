import type { MaybeRefOrGetter } from 'vue'

// long lists show the first `step` rows, the next ones are added when the end of the list scrolls into view.
// put `sentinel` (a template ref) on an empty element after the rows, and render it only while `hasMore`
export function useLazyList<T>(items: MaybeRefOrGetter<T[]>, step = 20) {
  const count = ref(step)
  const visible = computed(() => toValue(items).slice(0, count.value))
  const hasMore = computed(() => count.value < toValue(items).length)
  const sentinel = ref<HTMLElement>()

  // a different list (new filter, other month): start from the top again
  watch(() => toValue(items).length, () => { count.value = step })

  let observer: IntersectionObserver | undefined

  // the observer only fires when the element enters the screen, so after adding rows
  // look again: the end can still be in view on a tall screen
  function observe() {
    observer?.disconnect()
    if (sentinel.value) observer?.observe(sentinel.value)
  }

  onMounted(() => {
    observer = new IntersectionObserver((entries) => {
      if (!entries.some(entry => entry.isIntersecting) || !hasMore.value) return
      count.value += step
      nextTick(observe)
    }, { rootMargin: '300px' })
    observe()
  })
  onBeforeUnmount(() => observer?.disconnect())

  watch(sentinel, observe, { flush: 'post' })

  return { visible, hasMore, sentinel }
}
