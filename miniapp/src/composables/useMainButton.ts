import { watchEffect, onMounted, onUnmounted, unref, shallowRef, type Ref } from 'vue'
import { getWebApp } from '../telegram/webapp'

type MaybeRef<T> = Ref<T> | T

export interface MainButtonOptions {
  text: MaybeRef<string>
  visible?: Ref<boolean>
  enabled?: Ref<boolean>
  loading?: Ref<boolean>
  onClick: () => void | Promise<void>
}

// Реактивный владелец кнопки: последний смонтированный экран владеет ею.
// Реактивность нужна, чтобы watchEffect применил параметры после onMounted.
const activeOwner = shallowRef<symbol | null>(null)
let currentHandler: (() => void) | null = null

export function useMainButton(opts: MainButtonOptions): void {
  const owner = Symbol('main-button')
  let running = false

  const handler = (): void => {
    if (running) return // гвард от повторного тапа
    const enabled = opts.enabled ? unref(opts.enabled) : true
    const visible = opts.visible ? unref(opts.visible) : true
    if (!enabled || !visible) return
    const result = opts.onClick()
    if (result instanceof Promise) {
      running = true
      result.finally(() => {
        running = false
      })
    }
  }

  onMounted(() => {
    const tg = getWebApp()
    if (!tg) return
    if (currentHandler) tg.MainButton.offClick(currentHandler)
    currentHandler = handler
    tg.MainButton.onClick(handler)
    activeOwner.value = owner // триггерит watchEffect с применением параметров
  })

  watchEffect(() => {
    // Сначала читаем все реактивные значения — иначе эффект не пересчитается.
    const text = unref(opts.text)
    const enabled = opts.enabled ? unref(opts.enabled) : true
    const visible = opts.visible ? unref(opts.visible) : true
    const loading = opts.loading ? unref(opts.loading) : false
    const isOwner = activeOwner.value === owner

    const tg = getWebApp()
    if (!tg || !isOwner) return

    // Загрузка считается неактивным состоянием, но прогресс-спиннер Telegram
    // использует text_color — поэтому во время загрузки держим bone/obsidian.
    const dim = !enabled && !loading
    tg.MainButton.setParams({
      text,
      color: dim ? '#262626' : '#FFFFFF',
      text_color: dim ? '#686868' : '#161616',
      is_active: enabled && !loading,
      is_visible: visible,
    })
    if (loading) tg.MainButton.showProgress(false)
    else tg.MainButton.hideProgress()
  })

  onUnmounted(() => {
    const tg = getWebApp()
    if (!tg) return
    if (activeOwner.value === owner) {
      if (currentHandler) tg.MainButton.offClick(currentHandler)
      currentHandler = null
      activeOwner.value = null
      tg.MainButton.hide()
    }
  })
}
