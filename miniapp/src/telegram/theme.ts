// Тема Mini App: светлая и тёмная.
import { ref } from 'vue'
import { getWebApp, supports, type TgWebApp } from './webapp'

export type Theme = 'light' | 'dark'

// Цвет хрома Telegram (шапка, фон, нижняя панель) по теме — совпадает с --void.
export const CHROME = { light: '#ffffff', dark: '#0a0a0a' } as const

export const theme = ref<Theme>('dark')

// Выбор темы: явный colorScheme Telegram приоритетнее; иначе системная настройка.
export function resolveTheme(tg: { colorScheme?: string } | null, prefersLight: boolean): Theme {
  const cs = tg?.colorScheme
  if (cs === 'light' || cs === 'dark') return cs
  return prefersLight ? 'light' : 'dark'
}

// Цвета MainButton для темы и состояния (активна/загрузка vs неактивна).
export function mainButtonColors(
  t: Theme,
  s: { enabled: boolean; loading: boolean },
): { color: string; text_color: string } {
  const active = s.enabled || s.loading
  if (t === 'light') {
    return active ? { color: '#0A0A0A', text_color: '#FFFFFF' } : { color: '#E5E5E5', text_color: '#8A8A8A' }
  }
  return active ? { color: '#FFFFFF', text_color: '#161616' } : { color: '#262626', text_color: '#686868' }
}

function prefersLight(): boolean {
  try {
    return !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches)
  } catch {
    return false
  }
}

function syncChrome(tg: TgWebApp, t: Theme): void {
  const c = CHROME[t]
  try {
    if (supports('6.9')) tg.setHeaderColor(c)
  } catch {
    // старые клиенты поддерживают только keyword — пропускаем
  }
  try {
    if (supports('6.1')) tg.setBackgroundColor(c)
  } catch {
  }
  try {
    if (supports('7.10') && typeof tg.setBottomBarColor === 'function') tg.setBottomBarColor(c)
  } catch {
  }
}

// Пишет data-theme, meta theme-color/color-scheme и красит хром под активную тему.
function render(): void {
  const t = theme.value
  const root = document.documentElement
  root.dataset.theme = t
  root.style.colorScheme = t
  setMeta('theme-color', CHROME[t])
  setMeta('color-scheme', t)
  const tg = getWebApp()
  if (tg) syncChrome(tg, t)
}

function setMeta(name: string, content: string): void {
  let el = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.name = name
    document.head.appendChild(el)
  }
  el.content = content
}

let wired = false

// Вычисляет тему и применяет её; подписывается на смену темы (один раз).
export function applyTheme(): void {
  const tg = getWebApp()
  theme.value = resolveTheme(tg, prefersLight())
  render()

  if (wired) return
  wired = true

  if (tg) {
    try {
      tg.onEvent('themeChanged', () => {
        theme.value = resolveTheme(getWebApp(), prefersLight())
        render()
      })
    } catch {
    }
  } else {
    try {
      const mq = window.matchMedia('(prefers-color-scheme: light)')
      const onChange = (): void => {
        theme.value = resolveTheme(null, prefersLight())
        render()
      }
      if (mq.addEventListener) mq.addEventListener('change', onChange)
      else if (mq.addListener) mq.addListener(onChange)
    } catch {
    }
  }
}
