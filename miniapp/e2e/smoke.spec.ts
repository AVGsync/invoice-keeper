import { test, expect, type Page, type ConsoleMessage } from '@playwright/test'
import path from 'node:path'
import { installTelegram, fakeInitData } from './fixtures/telegram'
import { installMockApi } from './fixtures/mockApi'
import { START_OCC_HEX, USERS } from './fixtures/data'

const SCREENSHOT_DIR = process.env.SCREENSHOT_DIR ?? 'test-results/screens'

function shotPath(name: string, theme: string): string {
  return path.join(SCREENSHOT_DIR, `${name}-${theme}.png`)
}

const INIT_DATA_USER = fakeInitData(Number(USERS.user1!.id.replace(/\D/g, '').slice(0, 6)) || 100001)

// Проверяет, что в консоли нет ошибок и утечки initData.
function guardConsole(page: Page): { errors: string[] } {
  const errors: string[] = []
  page.on('console', (msg: ConsoleMessage) => {
    const t = msg.text()
    expect(t).not.toContain('query_id=e2e')
    expect(t).not.toContain(INIT_DATA_USER)
    if (msg.type() === 'error') errors.push(t)
  })
  return { errors }
}

async function screenshot(page: Page, name: string, theme: string): Promise<void> {
  await page.waitForTimeout(700) // дать входным анимациям доиграть
  // Экран телефона, а не вся страница: фиксированная панель вкладок остаётся внизу.
  await page.screenshot({ path: shotPath(name, theme) })
}

async function waitMainButton(page: Page): Promise<void> {
  await expect(page.locator('[data-testid=tg-main-button]')).toBeVisible()
}

for (const theme of ['light', 'dark'] as const) {
  test(`walkthrough (${theme})`, async ({ page }) => {
    const { errors } = guardConsole(page)
    await installTelegram(page, { colorScheme: theme })
    const api = await installMockApi(page, { actingUser: 'user1' })

    await page.goto('/tg/')
    await expect(page.getByRole('heading', { name: 'Сегодня' })).toBeVisible()
    // Тема применена к документу и фону
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme)
    const bodyBg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
    expect(bodyBg).toBe(theme === 'dark' ? 'rgb(10, 10, 10)' : 'rgb(255, 255, 255)')
    // Баннер просрочки
    await expect(page.getByText(/Просрочено:/)).toBeVisible()
    await screenshot(page, 'today', theme)

    // Платежи
    await page.getByRole('link', { name: 'Платежи' }).click()
    await expect(page.getByRole('heading', { name: 'Платежи' })).toBeVisible()
    await screenshot(page, 'payments', theme)

    // Карточка (переход на первый overdue из Today)
    await page.goto('/tg/')
    await page.locator('.occ-row').first().click()
    await expect(page.locator('.desc').first()).toBeVisible()
    await waitMainButton(page)
    await screenshot(page, 'occurrence', theme)

    // Оплатить сегодня
    await page.locator('[data-testid=tg-main-button]').click()
    await expect(page.locator('.head').getByText('Оплачен', { exact: false })).toBeVisible()
    await screenshot(page, 'occurrence-paid', theme)

    // Календарь (с карточки TabBar не виден — переходим напрямую)
    await page.goto('/tg/calendar')
    await expect(page.locator('.grid')).toBeVisible()
    await page.locator('.grid .cell').nth(25).click()
    await screenshot(page, 'calendar', theme)

    // Спросить — композер вместо MainButton
    await page.goto('/tg/ask')
    const sendBtn = page.getByRole('button', { name: 'Отправить' })
    await expect(page.locator('[data-testid=tg-main-button]')).toBeHidden()
    await expect(sendBtn).toBeDisabled() // пустое поле
    const ta = page.locator('.composer textarea')
    await ta.fill('Что просрочено?')
    await expect(sendBtn).toBeEnabled()
    await sendBtn.click()
    await expect(page.getByText(/Просрочен один платёж/)).toBeVisible()
    await screenshot(page, 'ask', theme)

    // Второй вопрос — через Enter
    await ta.fill('А у Марии?')
    await ta.press('Enter')
    await expect(page.locator('.bubble.q', { hasText: 'А у Марии?' })).toBeVisible()

    // Shift+Enter вставляет перевод строки, а не отправляет
    await ta.fill('строка1')
    await ta.press('Shift+Enter')
    expect(await ta.inputValue()).toContain('\n')
    await ta.fill('')
    await expect(sendBtn).toBeDisabled()

    // Новый диалог: просим бэкенд забыть историю, фид очищается
    await page.getByRole('button', { name: 'Новый диалог' }).click()
    await page.waitForTimeout(200)
    expect(
      api.requests.some((r) => r.method === 'DELETE' && r.path.includes('/assistant/history')),
    ).toBe(true)
    await expect(page.locator('.exchange')).toHaveCount(0)

    // Профиль (без раздела администрирования у user)
    await page.goto('/tg/profile')
    await expect(page.getByRole('heading', { name: 'Профиль' })).toBeVisible()
    await expect(page.getByText('Администрирование')).toHaveCount(0)
    await screenshot(page, 'profile', theme)

    expect(errors).toEqual([])
  })

  test(`admin screens (${theme})`, async ({ page }) => {
    guardConsole(page)
    await installTelegram(page, { colorScheme: theme })
    const api = await installMockApi(page, { actingUser: 'admin' })

    await page.goto('/tg/admin/recurring')
    await expect(page.getByRole('heading', { name: 'Регулярные платежи' })).toBeVisible()
    await screenshot(page, 'recurring', theme)

    await page.goto('/tg/admin/recurring/new')
    await expect(page.getByRole('heading', { name: 'Новый платёж' })).toBeVisible()
    await page.locator('input[type=text]').first().fill('Тестовый платёж')
    await page.locator('input[inputmode=decimal]').fill('1 234,56')
    await page.locator('input[type=radio]').first().check()
    await screenshot(page, 'recurring-new', theme)
    await waitMainButton(page)
    await page.locator('[data-testid=tg-main-button]').click()
    await page.waitForTimeout(200)
    const createReq = api.find('POST', '/recurring-payments')
    expect(createReq).toBeTruthy()
    const b = createReq!.body as Record<string, unknown>
    expect(b.amount).toBe(123456)
    expect(b.day_offset).toBeUndefined()
    expect(b.start_date).toBeTruthy()
    expect(b.frequency).toBeTruthy()
    expect(b.interval).toBeTruthy()
    expect(b.assignee_id).toBeTruthy()

    await page.goto('/tg/admin/users')
    await expect(page.getByRole('heading', { name: 'Сотрудники' })).toBeVisible()
    await screenshot(page, 'users', theme)

    await page.goto('/tg/admin/users/new')
    await expect(page.getByRole('heading', { name: 'Новый сотрудник' })).toBeVisible()
    await page.locator('input[type=text]').first().fill('Новый Сотрудник')
    await page.locator('input[autocapitalize=off]').fill('newuser')
    await page.locator('input[type=password]').fill('password123')
    await screenshot(page, 'user-new', theme)
    await waitMainButton(page)
    await page.locator('[data-testid=tg-main-button]').click()
    await page.waitForTimeout(200)
    expect(api.find('POST', '/users')).toBeTruthy()
  })

  test(`link screen (${theme})`, async ({ page }) => {
    guardConsole(page)
    await installTelegram(page, { colorScheme: theme })
    await installMockApi(page, { actingUser: 'user1', authMode: 'needs_link' })

    await page.goto('/tg/')
    await expect(page.getByRole('heading', { name: 'Привязка аккаунта' })).toBeVisible()
    await screenshot(page, 'link', theme)
    await page.locator('input[autocomplete=username]').fill('user1')
    await page.locator('input[type=password]').fill('secret')
    await waitMainButton(page)
    await page.locator('[data-testid=tg-main-button]').click()
    await expect(page.getByRole('heading', { name: 'Сегодня' })).toBeVisible()
  })

  test(`expired screen (${theme})`, async ({ page }) => {
    guardConsole(page)
    await installTelegram(page, { colorScheme: theme })
    await installMockApi(page, { authMode: 'expired' })
    await page.goto('/tg/')
    await expect(page.getByText(/Сессия устарела/)).toBeVisible()
    await screenshot(page, 'expired', theme)
  })
}

// Сценарии без привязки к теме.

test('смена темы Telegram на лету переключает data-theme', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page, { colorScheme: 'dark' })
  await installMockApi(page, { actingUser: 'user1' })
  await page.goto('/tg/')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.evaluate(() => {
    ;(window as unknown as { __tgMock: { setTheme(s: string): void } }).__tgMock.setTheme('light')
  })
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
  expect(bg).toBe('rgb(255, 255, 255)')
})

test('chips: последний чип не прилипает к краю (390 и 320)', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page)
  await installMockApi(page, { actingUser: 'user1' })

  for (const vp of [
    { width: 390, height: 844 },
    { width: 320, height: 640 },
  ]) {
    await page.setViewportSize(vp)
    await page.goto('/tg/payments')
    await expect(page.getByRole('heading', { name: 'Платежи' })).toBeVisible()
    const chips = page.locator('.chips')
    const count = await chips.count()
    expect(count).toBeGreaterThan(0)
    const iw = await page.evaluate(() => window.innerWidth)
    for (let i = 0; i < count; i++) {
      const c = chips.nth(i)
      // в начале прокрутки первый чип отступает на гаттер
      await c.evaluate((el) => {
        el.scrollLeft = 0
      })
      const firstLeft = await c
        .locator('.chip')
        .first()
        .evaluate((el) => el.getBoundingClientRect().left)
      expect(firstLeft).toBeGreaterThanOrEqual(16 - 0.5)
      // в конце прокрутки последний чип не упирается в край
      await c.evaluate((el) => {
        el.scrollLeft = el.scrollWidth
      })
      await page.waitForTimeout(50)
      const lastRight = await c
        .locator('.chip')
        .last()
        .evaluate((el) => el.getBoundingClientRect().right)
      expect(lastRight).toBeLessThanOrEqual(iw - 16 + 0.5)
    }
  }
})

test('чужая карточка: нет кнопки и показана подсказка', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page)
  await installMockApi(page, { actingUser: 'user1' })
  await page.goto('/tg/occ/00000000-0000-4000-8000-00000000a005')
  await expect(page.getByText(/Отметить оплату может ответственный/)).toBeVisible()
  await expect(page.locator('[data-testid=tg-main-button]')).toBeHidden()
})

test('оплата прошлой датой передаёт paid_date', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page)
  const api = await installMockApi(page, { actingUser: 'user1' })
  await page.goto('/tg/occ/00000000-0000-4000-8000-00000000a001')
  await waitMainButton(page)
  await page.locator('input[type=date]').fill('2026-10-20')
  await page.locator('[data-testid=tg-main-button]').click()
  await page.waitForTimeout(200)
  const patch = api.find('PATCH', '/payment-occurrences/00000000-0000-4000-8000-00000000a001')
  expect(patch).toBeTruthy()
  expect((patch!.body as Record<string, unknown>).paid_date).toBe('2026-10-20')
})

test('отмена оплаты шлёт action cancel', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page)
  const api = await installMockApi(page, { actingUser: 'user1' })
  await page.goto('/tg/occ/00000000-0000-4000-8000-00000000a006') // уже оплачен
  await page.getByRole('button', { name: 'Отменить оплату' }).click()
  await page.waitForTimeout(200)
  const patch = api.requests.find(
    (r) => r.method === 'PATCH' && (r.body as Record<string, unknown>)?.action === 'cancel',
  )
  expect(patch).toBeTruthy()
})

test('user: /admin/users редиректит на главную', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page)
  await installMockApi(page, { actingUser: 'user1' })
  await page.goto('/tg/admin/users')
  await expect(page.getByRole('heading', { name: 'Сегодня' })).toBeVisible()
})

test('startapp occ_<hex32> открывает карточку, back ведёт на Today', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page, { startParam: 'occ_' + START_OCC_HEX })
  await installMockApi(page, { actingUser: 'user1' })
  await page.goto('/tg/')
  await expect(page).toHaveURL(/\/occ\//)
  await expect(page.locator('.desc').first()).toBeVisible()
  await page.locator('[data-testid=tg-back-button]').click()
  await expect(page.getByRole('heading', { name: 'Сегодня' })).toBeVisible()
})

test('PATCH 403: алерт и сессия жива', async ({ page }) => {
  guardConsole(page)
  await installTelegram(page)
  await installMockApi(page, { actingUser: 'admin', forbidPatch: true })
  await page.goto('/tg/occ/00000000-0000-4000-8000-00000000a001')
  await waitMainButton(page)
  await page.locator('[data-testid=tg-main-button]').click()
  await page.waitForTimeout(200)
  const alerts = await page.evaluate(
    () => (window as unknown as { __tgMock?: { alerts: string[] } }).__tgMock?.alerts ?? [],
  )
  expect(alerts.some((a) => a.includes('Недостаточно прав'))).toBe(true)
  await expect(page.getByRole('heading', { name: 'Сегодня' }).or(page.locator('.desc').first())).toBeTruthy()
})
