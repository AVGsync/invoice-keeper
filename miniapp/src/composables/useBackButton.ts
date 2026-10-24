import { watch, onUnmounted, ref, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getWebApp, supports } from '../telegram/webapp'
import { session } from '../stores/session'

// Вызывается один раз в App.vue.
export function useBackButton(): { showInlineBack: Ref<boolean>; goBack: () => void } {
  const route = useRoute()
  const router = useRouter()
  const showInlineBack = ref(false)
  const available = supports('6.1')

  const goBack = (): void => {
    const parent = (route.meta.parent as string | undefined) ?? '/'
    if (history.state && history.state.back) router.back()
    else void router.replace(parent)
  }

  let handlerBound = false
  const bindHandler = (): void => {
    const tg = getWebApp()
    if (!tg || handlerBound) return
    tg.BackButton.onClick(goBack)
    handlerBound = true
  }

  watch(
    [() => route.fullPath, () => session.phase],
    () => {
      const tg = getWebApp()
      if (session.phase !== 'ready') {
        if (tg && available) tg.BackButton.hide()
        showInlineBack.value = false
        return
      }
      const isTab = !!route.meta.tab
      if (!available || !tg) {
        showInlineBack.value = !isTab
        return
      }
      bindHandler()
      if (isTab) tg.BackButton.hide()
      else tg.BackButton.show()
      showInlineBack.value = false
    },
    { immediate: true },
  )

  onUnmounted(() => {
    const tg = getWebApp()
    if (tg && handlerBound) tg.BackButton.offClick(goBack)
  })

  return { showInlineBack, goBack }
}
