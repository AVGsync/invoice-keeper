import { reactive, computed } from 'vue'
import { getWebApp, initSafeAreaEvents, haptic } from '../telegram/webapp'
import { applyTheme } from '../telegram/theme'
import { setToken, registerReauth } from '../api/client'
import { ApiError } from '../api/errors'
import { miniAppAuth, miniAppLink, getMe } from '../api/endpoints'
import type { MiniUser, User } from '../api/types'
import { parseStartParam } from '../telegram/startParam'
import { assistant } from './assistant'
import { occurrences } from './occurrences'

export type Phase =
  | 'booting'
  | 'link'
  | 'ready'
  | 'no_telegram'
  | 'expired'
  | 'invalid'
  | 'disabled'
  | 'rate_limited'
  | 'offline'

interface SessionState {
  phase: Phase
  user: MiniUser | null
  me: User | null
  dmAvailable: boolean
  fatalReason: string
  linkError: string
  linkLoading: boolean
}

export const session = reactive<SessionState>({
  phase: 'booting',
  user: null,
  me: null,
  dmAvailable: false,
  fatalReason: '',
  linkError: '',
  linkLoading: false,
})

export const isAdmin = computed(() => session.user?.role === 'admin')

// router и сторы инжектятся, чтобы не плодить циклические импорты.
type Router = import('vue-router').Router
let router: Router | null = null
let onReady: (() => void) | null = null

export function initSession(r: Router, readyCb: () => void): void {
  router = r
  onReady = readyCb
  registerReauth(reauth)
}

function currentInitData(): string {
  return getWebApp()?.initData ?? ''
}

async function applyAuth(resp: {
  access_token: string
  user: MiniUser
  dm_available: boolean
}): Promise<void> {
  setToken(resp.access_token)
  session.user = resp.user
  session.dmAvailable = resp.dm_available
  session.phase = 'ready'

  enforceAdminAccess()

  await Promise.allSettled([
    getMe()
      .then((m) => {
        session.me = m
      })
      .catch(() => {}),
    assistant.loadStatus().catch(() => {}),
    occurrences.load({ force: true }).catch(() => {}),
  ])

  onReady?.()
  await enforceRouteAccess()
  handleStartParam()
}

// Админ-маршрут недоступен обычному пользователю (роль известна сразу).
function enforceAdminAccess(): void {
  if (!router) return
  const rt = router
  void rt.isReady().then(() => {
    const r = rt.currentRoute.value
    if (r.meta.admin && !isAdmin.value) void rt.replace('/')
  })
}

// После авторизации проверяем доступ к текущему маршруту (прямой заход по ссылке мог пройти guard ещё до готовности сессии).
async function enforceRouteAccess(): Promise<void> {
  if (!router) return
  await router.isReady()
  const r = router.currentRoute.value
  if (r.meta.admin && !isAdmin.value) {
    void router.replace('/')
    return
  }
  if (r.name === 'ask' && !assistant.status.enabled) {
    void router.replace('/')
  }
}

export async function boot(): Promise<void> {
  const tg = getWebApp()
  if (!tg || !tg.initData) {
    session.phase = 'no_telegram'
    session.fatalReason = 'Откройте Invoice Keeper из Telegram'
    return
  }

  tg.ready()
  tg.expand()
  applyTheme()
  initSafeAreaEvents()

  await doAuth()
}

async function doAuth(): Promise<void> {
  const initData = currentInitData()
  try {
    const resp = await miniAppAuth(initData)
    await applyAuth(resp)
  } catch (e) {
    handleAuthError(e)
  }
}

function handleAuthError(e: unknown): void {
  if (e instanceof ApiError) {
    if (e.network || e.status === 0) {
      session.phase = 'offline'
      return
    }
    if (e.status === 409 && e.code === 'telegram_not_linked') {
      session.phase = 'link'
      return
    }
    if (e.status === 401) {
      if (e.code === 'init_data_expired') {
        session.phase = 'expired'
        session.fatalReason = 'Сессия устарела. Закройте и откройте приложение заново'
      } else {
        session.phase = 'invalid'
        session.fatalReason = 'Не удалось войти через Telegram'
      }
      return
    }
    if (e.status === 404 || e.status === 503) {
      session.phase = 'disabled'
      session.fatalReason = 'Вход через Telegram сейчас выключен'
      return
    }
    if (e.status === 429) {
      session.phase = 'rate_limited'
      session.fatalReason = 'Слишком много попыток, подождите минуту'
      return
    }
    if (e.status >= 500) {
      // Временная ошибка сервера — даём «Повторить».
      session.phase = 'offline'
      session.fatalReason = 'Сервис временно недоступен, попробуйте ещё раз'
      return
    }
  }
  session.phase = 'invalid'
  session.fatalReason = 'Не удалось войти'
}

export async function retryBoot(): Promise<void> {
  session.phase = 'booting'
  await doAuth()
}

export async function link(login: string, password: string): Promise<void> {
  session.linkError = ''
  session.linkLoading = true
  try {
    const resp = await miniAppLink(currentInitData(), login, password)
    await applyAuth(resp)
    session.linkError = ''
  } catch (e) {
    haptic.error()
    if (e instanceof ApiError) {
      if (e.status === 401 && e.code === 'init_data_expired') {
        session.phase = 'expired'
        session.fatalReason = 'Сессия устарела. Закройте и откройте приложение заново'
        return
      }
      if (e.code === 'invalid_credentials') {
        session.linkError = 'Неверный логин или пароль'
        return
      }
      if (e.code === 'telegram_already_linked') {
        session.linkError =
          'Этот Telegram уже привязан к другому аккаунту. Отвяжите его там (/unlink в боте)'
        return
      }
      if (e.status === 429) {
        session.linkError = 'Слишком много попыток, подождите минуту'
        return
      }
    }
    session.linkError = 'Не удалось войти, попробуйте ещё раз'
  } finally {
    session.linkLoading = false
  }
}

// Единый reauth (single-flight): повторяет POST /auth с тем же initData.
let reauthInflight: Promise<void> | null = null

export function reauth(): Promise<void> {
  if (reauthInflight) return reauthInflight
  reauthInflight = (async () => {
    try {
      const resp = await miniAppAuth(currentInitData())
      setToken(resp.access_token)
      session.user = resp.user
      session.dmAvailable = resp.dm_available
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.network || e.status === 0 || e.status >= 500) {
          session.phase = 'offline'
          session.fatalReason = 'Сервис временно недоступен, попробуйте ещё раз'
        } else if (e.status === 401 && e.code === 'init_data_expired') {
          session.phase = 'expired'
          session.fatalReason = 'Сессия устарела. Закройте и откройте приложение заново'
        } else if (e.status === 409 && e.code === 'telegram_not_linked') {
          session.phase = 'link'
        } else if (e.status === 429) {
          session.phase = 'rate_limited'
          session.fatalReason = 'Слишком много попыток, подождите минуту'
        } else {
          session.phase = 'invalid'
          session.fatalReason = 'Не удалось войти'
        }
      } else {
        session.phase = 'invalid'
        session.fatalReason = 'Не удалось войти'
      }
      throw e
    }
  })().finally(() => {
    reauthInflight = null
  })
  return reauthInflight
}

function handleStartParam(): void {
  const tg = getWebApp()
  const raw =
    tg?.initDataUnsafe.start_param ??
    new URLSearchParams(location.hash.slice(1)).get('tgWebAppStartParam') ??
    new URLSearchParams(location.search).get('tgWebAppStartParam')
  const parsed = parseStartParam(raw)
  if (parsed && router) {
    void router.replace('/').then(() => router?.push(`/occ/${parsed.occId}`))
  }
}
