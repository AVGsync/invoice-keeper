// Типизированная обёртка над Telegram.WebApp (подмножество используемого).

export interface ThemeParams {
  bg_color?: string
  text_color?: string
  hint_color?: string
  link_color?: string
  button_color?: string
  button_text_color?: string
  secondary_bg_color?: string
  section_bg_color?: string
  section_header_text_color?: string
  destructive_text_color?: string
}

export interface SafeAreaInset {
  top: number
  right: number
  bottom: number
  left: number
}

export interface TgMainButton {
  setText(text: string): TgMainButton
  setParams(p: {
    text?: string
    color?: string
    text_color?: string
    is_active?: boolean
    is_visible?: boolean
  }): TgMainButton
  show(): TgMainButton
  hide(): TgMainButton
  enable(): TgMainButton
  disable(): TgMainButton
  showProgress(leaveActive?: boolean): TgMainButton
  hideProgress(): TgMainButton
  onClick(cb: () => void): TgMainButton
  offClick(cb: () => void): TgMainButton
}

export interface TgBackButton {
  show(): TgBackButton
  hide(): TgBackButton
  onClick(cb: () => void): TgBackButton
  offClick(cb: () => void): TgBackButton
}

export interface TgHaptic {
  impactOccurred(style: 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'): TgHaptic
  notificationOccurred(type: 'error' | 'success' | 'warning'): TgHaptic
  selectionChanged(): TgHaptic
}

export interface TgWebApp {
  initData: string
  initDataUnsafe: {
    user?: { id: number; first_name?: string; username?: string; allows_write_to_pm?: boolean }
    start_param?: string
  }
  version: string
  platform: string
  colorScheme: 'light' | 'dark'
  themeParams: ThemeParams
  isVersionAtLeast(v: string): boolean
  ready(): void
  expand(): void
  close(): void
  MainButton: TgMainButton
  BackButton: TgBackButton
  HapticFeedback: TgHaptic
  openLink?(url: string, options?: { try_instant_view?: boolean }): void
  showAlert(message: string, cb?: () => void): void
  showConfirm(message: string, cb?: (ok: boolean) => void): void
  enableClosingConfirmation(): void
  disableClosingConfirmation(): void
  setHeaderColor(color: string): void
  setBackgroundColor(color: string): void
  setBottomBarColor?(color: string): void
  onEvent(name: string, cb: () => void): void
  offEvent(name: string, cb: () => void): void
  safeAreaInset?: SafeAreaInset
  contentSafeAreaInset?: SafeAreaInset
  disableVerticalSwipes?(): void
  enableVerticalSwipes?(): void
}

declare global {
  interface Window {
    Telegram?: { WebApp?: TgWebApp }
  }
}

export function getWebApp(): TgWebApp | null {
  return window.Telegram?.WebApp ?? null
}

export function supports(v: string): boolean {
  try {
    return getWebApp()?.isVersionAtLeast(v) ?? false
  } catch {
    return false
  }
}

export const haptic = {
  success(): void {
    if (supports('6.1')) getWebApp()?.HapticFeedback.notificationOccurred('success')
  },
  error(): void {
    if (supports('6.1')) getWebApp()?.HapticFeedback.notificationOccurred('error')
  },
  warning(): void {
    if (supports('6.1')) getWebApp()?.HapticFeedback.notificationOccurred('warning')
  },
  selection(): void {
    if (supports('6.1')) getWebApp()?.HapticFeedback.selectionChanged()
  },
  impact(style: 'light' | 'medium' | 'heavy' = 'light'): void {
    if (supports('6.1')) getWebApp()?.HapticFeedback.impactOccurred(style)
  },
}

export function confirm(message: string): Promise<boolean> {
  const tg = getWebApp()
  if (tg && supports('6.2')) {
    return new Promise((resolve) => {
      tg.showConfirm(message, (ok) => resolve(!!ok))
    })
  }
  return Promise.resolve(window.confirm(message))
}

export function alert(message: string): Promise<void> {
  const tg = getWebApp()
  if (tg && supports('6.2')) {
    return new Promise((resolve) => {
      tg.showAlert(message, () => resolve())
    })
  }
  window.alert(message)
  return Promise.resolve()
}

// Открывает адрес во внешнем браузере (в Telegram — через openLink).
export function openExternal(url: string): void {
  const tg = getWebApp()
  if (tg?.openLink) tg.openLink(url)
  else window.open(url, '_blank', 'noopener')
}

export function setClosingConfirmation(on: boolean): void {
  const tg = getWebApp()
  if (!tg || !supports('6.2')) return
  if (on) tg.enableClosingConfirmation()
  else tg.disableClosingConfirmation()
}

// Тема и покраска хрома Telegram вынесены в ./theme.ts (applyTheme).
export function initSafeAreaEvents(): void {
  const tg = getWebApp()
  if (!tg) return
  tg.onEvent('safeAreaChanged', () => {
  })
  tg.onEvent('contentSafeAreaChanged', () => {
  })
}
