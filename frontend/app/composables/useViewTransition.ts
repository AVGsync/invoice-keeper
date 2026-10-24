import type { Ref } from 'vue'

// runs a state change through the browser's View Transitions API:
// blocks with a `view-transition-name` morph to their new size/place and their content crossfades.
// browsers without it, or people with reduced motion on, just get the change instantly.
// the look (duration, what stays still) is set in main.css
export function useViewTransition() {
  const { reduceMotion } = usePrefs()

  function withTransition(update: () => void) {
    if (!import.meta.client || !document.startViewTransition || reduceMotion.value) {
      update()
      return
    }
    document.startViewTransition(async () => {
      update()
      await nextTick()
    })
  }

  // a v-model friendly copy of a ref: every write goes through withTransition
  function transitioned<T>(source: Ref<T>) {
    return computed({
      get: () => source.value,
      set: (value: T) => withTransition(() => { source.value = value }),
    })
  }

  return { withTransition, transitioned }
}
