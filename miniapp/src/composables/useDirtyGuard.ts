import { watch, onUnmounted, type Ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { setClosingConfirmation, confirm } from '../telegram/webapp'

// Защита от потери несохранённых данных.
export function useDirtyGuard(
  dirty: Ref<boolean>,
  submitted?: Ref<boolean>,
): void {
  watch(
    dirty,
    (v) => {
      setClosingConfirmation(v)
    },
    { immediate: true },
  )

  onBeforeRouteLeave(async () => {
    if (dirty.value && !(submitted && submitted.value)) {
      const ok = await confirm('Выйти без сохранения?')
      if (!ok) return false
    }
    return true
  })

  onUnmounted(() => {
    setClosingConfirmation(false)
  })
}
