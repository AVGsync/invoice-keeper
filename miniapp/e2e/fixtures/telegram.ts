import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import type { Page } from '@playwright/test'
import { USERS } from './data'

const here = dirname(fileURLToPath(import.meta.url))
const mockSource = readFileSync(resolve(here, '../../dev/telegram-mock.js'), 'utf-8')

export interface TelegramOptions {
  colorScheme?: 'light' | 'dark'
  startParam?: string
  userId?: number
}

// Фейковый, но непустой initData (валидность проверяет мок-API, а не подпись).
export function fakeInitData(userId: number): string {
  const user = JSON.stringify({ id: userId, first_name: 'E2E', username: 'e2e', allows_write_to_pm: true })
  return `query_id=e2e&user=${encodeURIComponent(user)}&auth_date=1790000000&hash=e2e`
}

export async function installTelegram(page: Page, opts: TelegramOptions = {}): Promise<void> {
  const colorScheme = opts.colorScheme ?? 'light'
  const userId = opts.userId ?? (Number(USERS.user1!.id.replace(/\D/g, '').slice(0, 6)) || 100001)
  const initData = fakeInitData(userId)

  // Фиксируем время: 2026-10-24 09:00 МСК.
  await page.clock.setFixedTime(new Date('2026-10-24T09:00:00+03:00'))

  // Подменяем настоящий SDK Telegram моком.
  await page.route('https://telegram.org/js/telegram-web-app.js', (route) =>
    route.fulfill({ contentType: 'application/javascript', body: mockSource }),
  )

  await page.addInitScript(
    ([cfg]) => {
      ;(window as unknown as { __TG_MOCK_CONFIG__: unknown }).__TG_MOCK_CONFIG__ = cfg
    },
    [{ initData, startParam: opts.startParam, colorScheme }],
  )

  // Любые прочие внешние запросы — обрываем.
  await page.route('**/*', (route) => {
    const url = route.request().url()
    if (url.includes('/api/v1/')) return route.fallback()
    if (url.startsWith('http://localhost') || url.startsWith('https://telegram.org')) {
      return route.fallback()
    }
    return route.abort()
  })
}
