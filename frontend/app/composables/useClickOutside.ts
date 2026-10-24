import type { MaybeRefOrGetter } from 'vue'

// closes a popup (menu, calendar, popover) on a press anywhere outside `target`, or on Escape.
// `active` says whether the popup is open right now, so closed popups ignore the clicks
export function useClickOutside(
  target: MaybeRefOrGetter<HTMLElement | null | undefined>,
  close: () => void,
  active: MaybeRefOrGetter<boolean> = true,
) {
  function onPointerDown(e: PointerEvent) {
    const el = toValue(target)
    if (toValue(active) && el && !el.contains(e.target as Node)) close()
  }

  function onKeyDown(e: KeyboardEvent) {
    if (toValue(active) && e.key === 'Escape') close()
  }

  onMounted(() => {
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
  })
  onBeforeUnmount(() => {
    document.removeEventListener('pointerdown', onPointerDown)
    document.removeEventListener('keydown', onKeyDown)
  })
}
