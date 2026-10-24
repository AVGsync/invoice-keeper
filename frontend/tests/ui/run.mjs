// end-to-end ui checks, ids match TESTING.md.
// needs `npm run dev` running. creates uitest_* users/personas on the live server and deletes them at the end.
//   npm run test:ui
// env: UI_BASE (default http://localhost:3000), UI_ADMIN_LOGIN, UI_ADMIN_PASSWORD (default: values from .env.example)
import { chromium } from 'playwright'
import { mkdirSync, writeFileSync } from 'node:fs'

const BASE = process.env.UI_BASE ?? 'http://localhost:3000'
const SHOTS = new URL('./shots/', import.meta.url).pathname
const ADMIN = { login: process.env.UI_ADMIN_LOGIN ?? 'admin', password: process.env.UI_ADMIN_PASSWORD ?? 'admin123' }
mkdirSync(SHOTS, { recursive: true })
const tag = Math.random().toString(36).slice(2, 7)
const P1 = `uitest_p_${tag}`
const P2 = `uitest_p2_${tag}`
const LOGIN = `uitest_${tag}`

const results = {}
// set once the page exists: screenshot of the screen where a check failed
let onFail = async () => {}
const consoleIssues = []

async function check(id, fn) {
  try {
    const note = await fn()
    results[id] = { ok: true, note: note ?? '' }
  }
  catch (e) {
    results[id] = { ok: false, note: String(e.message ?? e).replace(/\s+/g, ' ').slice(0, 400) }
    await onFail(id)
  }
  console.log(`${results[id].ok ? 'PASS' : 'FAIL'} ${id} ${results[id].note}`)
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function watch(page, who) {
  page.on('console', (msg) => {
    if (msg.type() === 'error' || /hydration/i.test(msg.text())) consoleIssues.push(`${who} ${page.url()} [${msg.type()}] ${msg.text()}`)
  })
  page.on('pageerror', e => consoleIssues.push(`${who} ${page.url()} [pageerror] ${e.message}`))
}

const browser = await chromium.launch()
// most checks are about the screens, so they run on the fixed demo data (demo mode on).
// T48 checks the live api calls, T50 the demo actions
const demoOn = () => {
  if (!localStorage.getItem('invoice-keeper:prefs')) localStorage.setItem('invoice-keeper:prefs', JSON.stringify({ demoMode: true }))
}
const admin = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
await admin.addInitScript(demoOn)
const page = await admin.newPage()
watch(page, 'admin')
const shot = name => page.screenshot({ path: `${SHOTS}${name}.png` })
const rows = () => page.locator('.table .row:not(.head)')
const rowsOf = p => p.locator('.table .row:not(.head)')
const modalTitled = (p, title) => p.locator('.modal').filter({ has: p.locator('h2.title', { hasText: new RegExp(`^${title}$`) }) })
// networkidle sometimes never comes through the dev proxy (a finished api answer stays "open" for playwright),
// so it gets a short wait; the checks after it wait for their own elements anyway
const hydrated = p => p.waitForFunction(() => !!document.querySelector('#__nuxt')?.__vue_app__, null, { timeout: 15000 })
  .then(() => p.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {}))
let userPassword = ''

// date fields can't be typed into: open the calendar, go to the month, click the day
const monthNames = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
async function pickDate(input, iso) {
  const [year, month, day] = iso.split('-').map(Number)
  const calendar = input.locator('xpath=..').locator('.calendar')
  await input.click()
  await calendar.waitFor({ timeout: 5000 })
  for (let i = 0; i < 36; i++) {
    const [name, shownYear] = (await calendar.locator('.month').innerText()).split(' ')
    const diff = (year - Number(shownYear)) * 12 + (month - 1 - monthNames.indexOf(name))
    if (diff === 0) break
    await calendar.getByRole('button', { name: diff > 0 ? 'Следующий месяц' : 'Предыдущий месяц' }).click()
  }
  await calendar.locator('.day:not(.other)', { hasText: new RegExp(`^${day}$`) }).click()
  await calendar.waitFor({ state: 'detached', timeout: 5000 })
}
onFail = id => page.screenshot({ path: `${SHOTS}fail-${id}.png` }).catch(() => {})

try {
  // ---------- auth & routing ----------
  await check('T1', async () => {
    // first load after a code change can be slow while the dev server compiles
    await page.goto(`${BASE}/admin/users`, { timeout: 60000 })
    await page.waitForURL('**/login', { timeout: 30000 })
  })

  await check('T2', async () => {
    await page.goto(`${BASE}/`)
    await page.waitForURL('**/login', { timeout: 15000 })
  })

  await check('T25', async () => {
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
    await shot('C1-login')
    const title = await page.locator('.receipt .line').first().locator('.title').boundingBox()
    const amount = await page.locator('.receipt .line').first().locator('.amount').boundingBox()
    const receipt = await page.locator('.receipt').boundingBox()
    const stamp = await page.locator('.receipt .stamp').boundingBox()
    assert(title.x < receipt.x + receipt.width / 2, `title not on the left (x=${title.x})`)
    assert(amount.x - (title.x + title.width) >= 16, `gap title→amount ${Math.round(amount.x - title.x - title.width)}px`)
    assert(stamp.y + stamp.height <= receipt.y + receipt.height && stamp.x >= receipt.x && stamp.x + stamp.width <= receipt.x + receipt.width, 'stamp sticks out of the receipt')
    return `gap ${Math.round(amount.x - title.x - title.width)}px, stamp inside`
  })

  const loginWith = async (p, creds) => {
    await p.locator('form input').first().fill(creds.login)
    await p.locator('form input[type="password"]').fill(creds.password)
    await p.getByRole('button', { name: 'Войти' }).click()
  }

  await check('T3', async () => {
    await hydrated(page)
    await loginWith(page, { login: 'admin', password: 'wrong-password-1' })
    // right after many test logins the server may answer 429 (rate limit) instead
    await page.getByText(/Неверный логин или пароль|Слишком много попыток входа/).waitFor({ timeout: 10000 })
    return await page.locator('form .error-text').innerText()
  })

  await check('T4', async () => {
    await loginWith(page, ADMIN)
    await page.waitForURL('**/admin/overview', { timeout: 15000 })
    const name = await page.locator('.me-name').innerText()
    const role = await page.locator('.me-role').innerText()
    assert(name === 'Администратор', `name "${name}"`)
    assert(role === 'admin · .env', `role "${role}"`)
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
    await shot('A1-overview')
  })

  await check('T9', async () => {
    await page.reload()
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
    assert(page.url().endsWith('/admin/overview'), `url after reload ${page.url()}`)
    assert(await page.locator('.me-name').innerText() === 'Администратор', 'sidebar empty after reload')
  })

  await check('T6', async () => {
    await page.goto(`${BASE}/login`)
    await page.waitForURL('**/admin/overview', { timeout: 15000 })
  })

  await check('T5', async () => {
    await page.goto(`${BASE}/user/payments`)
    await hydrated(page)
    await page.waitForURL('**/admin/payments', { timeout: 15000 })
  })

  // ---------- demo screens ----------
  await check('T18', async () => {
    const seg = page.locator('.filters .seg')
    const text = (await seg.innerText()).replace(/\s+/g, ' ')
    assert(/Все 7/.test(text) && /Не оплачено 4/.test(text) && /Оплачено 3/.test(text) && /Просрочено 1/.test(text), `counts "${text}"`)
    const expect = { 'Все': 7, 'Не оплачено': 4, 'Оплачено': 3, 'Просрочено': 1 }
    for (const [label, n] of Object.entries(expect)) {
      await seg.getByRole('button', { name: new RegExp(`^${label}`) }).click()
      const got = await rows().count()
      assert(got === n, `${label}: ${got} rows`)
    }
    await seg.getByRole('button', { name: /^Все/ }).click()
  })

  await check('T40', async () => {
    // currency chips: multiple choice, rows + counts + footer follow, the last chip can't be switched off
    const chip = code => page.locator('.filters .chip', { hasText: code })
    await chip('USD').click()
    await chip('EUR').click()
    assert(await rows().count() === 3, `RUB only: ${await rows().count()} rows`)
    const foot = await page.locator('.foot').innerText()
    assert(foot.includes('RUB') && !foot.includes('USD') && !foot.includes('EUR'), `footer "${foot.replace(/\s+/g, ' ')}"`)
    assert((await page.locator('.filters .seg').innerText()).replace(/\s+/g, ' ').includes('Все 3'), 'status counts ignore the currency filter')
    await chip('RUB').click()
    assert(await chip('RUB').getAttribute('aria-pressed') === 'true', 'last chip switched off')
    await chip('USD').click()
    await chip('EUR').click()
    assert(await rows().count() === 7, 'all currencies back: wrong row count')
  })

  await check('T41', async () => {
    // assignee dropdown
    await page.locator('.assignee .control').click()
    await page.locator('.assignee .menu .option', { hasText: 'Д. Ким' }).click()
    assert(await rows().count() === 3, `Д. Ким: ${await rows().count()} rows`)
    assert((await page.locator('.assignee .control').innerText()).includes('Д. Ким'), 'control does not show the pick')
    await shot('A2-payments-filtered')
    await page.locator('.assignee .control').click()
    await page.locator('.assignee .menu .option', { hasText: 'все' }).click()
    assert(await rows().count() === 7, 'back to everyone: wrong row count')
  })

  await check('T43', async () => {
    // filters that match nothing explain it and offer a reset
    const chip = code => page.locator('.filters .chip', { hasText: code })
    await chip('RUB').click()
    await chip('USD').click()
    await page.locator('.assignee .control').click()
    await page.locator('.assignee .menu .option', { hasText: 'Д. Ким' }).click()
    assert(await rows().count() === 0, `expected no rows, got ${await rows().count()}`)
    await page.getByText('Под эти фильтры нет ни одного платежа.').waitFor({ timeout: 5000 })
    await shot('A2-payments-empty')
    await page.getByRole('button', { name: 'Сбросить фильтры' }).click()
    assert(await rows().count() === 7, 'reset did not bring all rows back')
    for (const code of ['RUB', 'USD', 'EUR']) assert(await chip(code).getAttribute('aria-pressed') === 'true', `${code} still off after reset`)
  })

  await check('T45', async () => {
    // month picker on Платежи: 12-month dropdown, another month is empty, arrows step back
    const picker = page.locator('.filters .month-picker')
    await picker.locator('.label').click()
    const grid = picker.locator('.dropdown')
    await grid.waitFor({ timeout: 5000 })
    await grid.getByRole('button', { name: 'Ноя' }).click()
    await grid.waitFor({ state: 'detached', timeout: 5000 })
    assert((await picker.innerText()).includes('Ноябрь 2026'), `label "${await picker.innerText()}"`)
    assert(await rows().count() === 0, 'November should be empty in the demo data')
    await page.getByText('Под эти фильтры нет ни одного платежа.').waitFor({ timeout: 5000 })
    await picker.getByRole('button', { name: 'Предыдущий месяц' }).click()
    assert(await rows().count() === 7, 'back to October: wrong row count')
    assert((await page.locator('.foot').innerText()).includes('Итого за октябрь'), 'footer month')
  })

  await check('T19', async () => {
    await rows().filter({ hasText: 'Сервер VPS' }).click()
    const popover = page.locator('.popover')
    await popover.waitFor({ timeout: 5000 })
    assert((await popover.innerText()).includes('Сервер VPS'), 'popover shows wrong payment')
    // visible is not enough: nothing (rows, footer) may cover it
    const onTop = await popover.evaluate((el) => {
      const box = el.getBoundingClientRect()
      return el.contains(document.elementFromPoint(box.left + box.width / 2, box.bottom - 12))
    })
    assert(onTop, 'popover is covered by the rows/footer below it')
    await shot('A2-payments-popover')
    await popover.getByRole('button', { name: 'Отмена' }).click()
    await popover.waitFor({ state: 'detached', timeout: 5000 })
    // a paid row opens too: to fix the date or take the mark off
    await rows().filter({ hasText: 'Интернет в офисе' }).click()
    await popover.waitFor({ timeout: 5000 })
    assert(await popover.getByRole('button', { name: 'Снять отметку' }).count() === 1, 'paid row popover has no «Снять отметку»')
    await page.keyboard.press('Escape')
    await page.locator('h1.title').click()
    await popover.waitFor({ state: 'detached', timeout: 5000 })
  })

  await check('T21', async () => {
    for (const path of ['/admin/payments', '/admin/overview', '/admin/recurring']) {
      await page.goto(`${BASE}${path}`)
      await hydrated(page)
      await page.getByRole('button', { name: 'Новый платёж' }).click()
      const modal = page.locator('.modal')
      await modal.waitFor({ timeout: 5000 })
      assert((await modal.innerText()).includes('Новый регулярный платёж'), `${path}: wrong modal`)
      if (path === '/admin/recurring') await shot('A3-recurring-modal')
      await modal.getByRole('button', { name: '×' }).click()
      await modal.waitFor({ state: 'detached', timeout: 5000 })
    }
  })

  await check('T22a', async () => {
    await page.goto(`${BASE}/admin/recurring`)
    await hydrated(page)
    const stop = await page.getByRole('button', { name: 'Прекратить' }).count()
    assert(stop === 7, `Прекратить=${stop}`)
    assert(await page.locator('.read-only').count() === 0, 'admin sees read-only note')
  })

  await check('T32', async () => {
    // stopped payments hidden by default, the switch shows them
    assert(await page.getByText('Старый CRM-сервис').count() === 0, 'stopped payment visible by default')
    await page.getByRole('switch', { name: /Показать прекращённые/ }).click()
    await page.getByText('Старый CRM-сервис').waitFor({ timeout: 5000 })
    assert(await page.getByRole('button', { name: 'История' }).count() === 1, 'no «История» on the stopped row')
    await page.getByRole('switch', { name: /Показать прекращённые/ }).click()
    assert(await page.getByText('Старый CRM-сервис').count() === 0, 'switch did not hide it again')
  })

  const spaces = text => text.replace(/\s/g, ' ')

  await check('T33', async () => {
    // date picker + next dates preview with the last-day-of-month rule
    await page.getByRole('button', { name: 'Новый платёж' }).click()
    const modal = modalTitled(page, 'Новый регулярный платёж')
    const date = modal.getByLabel('Дата ближайшей оплаты')
    await date.click()
    const calendar = modal.locator('.calendar')
    await calendar.waitFor({ timeout: 5000 })
    await calendar.locator('.day:not(.other)', { hasText: /^15$/ }).click()
    await calendar.waitFor({ state: 'detached', timeout: 5000 })
    const picked = await date.locator('.value').innerText()
    assert(picked === '15.10.2026', `picked "${picked}"`)
    // T42: typing does nothing, the field is pick-only
    const typedBefore = await date.locator('.value').innerText()
    await date.focus()
    await page.keyboard.type('01.01.2030')
    const typedAfter = await date.locator('.value').innerText()
    const tag = await date.evaluate(el => el.tagName)
    results.T42 = typedAfter === typedBefore && tag === 'BUTTON'
      ? { ok: true, note: 'pick-only button' }
      : { ok: false, note: `typing changed the field to "${typedAfter}" (${tag})` }
    console.log(`${results.T42.ok ? 'PASS' : 'FAIL'} T42 ${results.T42.note}`)
    await pickDate(date, '2027-01-31')
    const preview = spaces(await modal.locator('.dates').innerText())
    assert(preview.startsWith('31.01.2027 28.02.2027 31.03.2027 30.04.2027'), `preview "${preview}"`)
    await shot('A3-date-picker')
    return preview
  })

  await check('T34', async () => {
    // money filter while typing, formatting on blur
    const modal = modalTitled(page, 'Новый регулярный платёж')
    const amount = modal.getByLabel('Сумма')
    await amount.pressSequentially('abc12345.678')
    assert(await amount.inputValue() === '12345,67', `typed "${await amount.inputValue()}"`)
    await amount.blur()
    const formatted = spaces(await amount.inputValue())
    assert(formatted === '12 345,67', `formatted "${formatted}"`)
    await modal.getByRole('button', { name: '×' }).click()
    return formatted
  })

  await check('T35', async () => {
    // totals for a chosen date on Обзор
    await page.goto(`${BASE}/admin/overview`)
    await hydrated(page)
    const rub = () => page.locator('article.currency', { hasText: 'RUB' }).locator('.sum').innerText().then(spaces)
    const before = await rub()
    // T38: the calendar sits at the right edge here, it must open leftwards and stay on screen
    await page.locator('.as-of .input').click()
    const calendar = page.locator('.as-of .calendar')
    await calendar.waitFor({ timeout: 5000 })
    await page.waitForTimeout(250)
    const box = await calendar.boundingBox()
    const view = page.viewportSize()
    results.T38 = box.x >= 0 && box.y >= 0 && box.x + box.width <= view.width && box.y + box.height <= view.height
      ? { ok: true, note: `calendar at ${Math.round(box.x)},${Math.round(box.y)}` }
      : { ok: false, note: `calendar off screen: ${JSON.stringify(box)}` }
    console.log(`${results.T38.ok ? 'PASS' : 'FAIL'} T38 ${results.T38.note}`)
    await page.keyboard.press('Escape')
    await page.locator('h1.title').click()
    await calendar.waitFor({ state: 'detached', timeout: 5000 })
    assert(before.startsWith('121 500,00'), `24.10: "${before}"`)
    const picker = page.locator('.as-of .input')
    await pickDate(picker, '2026-10-31')
    await page.waitForFunction(() => document.querySelector('article.currency .sum')?.textContent?.replace(/\s/g, ' ').startsWith('123 490,00'), null, { timeout: 5000 })
    assert((await page.locator('article.currency', { hasText: 'RUB' }).innerText()).includes('31.10.2026'), 'card label not updated')
    return `${before} → ${await rub()}`
  })

  await check('T37', async () => {
    // paying after the due date warns about «с опозданием»
    await page.goto(`${BASE}/admin/payments`)
    await hydrated(page)
    await rows().filter({ hasText: 'Сервер VPS' }).click()
    const popover = page.locator('.popover')
    assert(await popover.locator('.hint.late').count() === 0, 'late warning shown for an on-time date')
    await pickDate(popover.getByLabel('Фактическая дата оплаты'), '2026-10-27')
    await popover.locator('.hint.late').waitFor({ timeout: 5000 })
    await popover.getByRole('button', { name: 'Отмена' }).click()
  })

  await check('T39', async () => {
    // popups close on a press outside them and on Escape
    await rows().filter({ hasText: 'Сервер VPS' }).click()
    const popover = page.locator('.popover')
    await popover.waitFor({ timeout: 5000 })
    await page.locator('h1.title').click()
    await popover.waitFor({ state: 'detached', timeout: 5000 })

    await page.getByRole('button', { name: 'Новый платёж' }).click()
    const modal = modalTitled(page, 'Новый регулярный платёж')
    await modal.getByLabel('Периодичность').click()
    await modal.locator('.menu').waitFor({ timeout: 5000 })
    await modal.locator('h2.title').click()
    await modal.locator('.menu').waitFor({ state: 'detached', timeout: 5000 })

    await modal.getByLabel('Дата ближайшей оплаты').click()
    await modal.locator('.calendar').waitFor({ timeout: 5000 })
    await page.keyboard.press('Escape')
    await modal.locator('.calendar').waitFor({ state: 'detached', timeout: 5000 })
    await modal.getByRole('button', { name: '×' }).click()
  })

  await check('T46', async () => {
    // date picker: month name opens the 12-month view, picking a month narrows to its days
    await page.getByRole('button', { name: 'Новый платёж' }).click()
    const modal = modalTitled(page, 'Новый регулярный платёж')
    const date = modal.getByLabel('Дата ближайшей оплаты')
    await date.click()
    const calendar = modal.locator('.calendar')
    await calendar.locator('.head .month').click()
    await calendar.locator('.month-grid').waitFor({ timeout: 5000 })
    await calendar.getByRole('button', { name: 'Следующий год' }).click()
    await calendar.getByRole('button', { name: 'Янв' }).click()
    assert((await calendar.locator('.head .month').innerText()) === 'Январь 2027', `head "${await calendar.locator('.head .month').innerText()}"`)
    await calendar.locator('.day:not(.other)', { hasText: /^15$/ }).click()
    assert(await date.locator('.value').innerText() === '15.01.2027', `picked "${await date.locator('.value').innerText()}"`)
    await modal.getByRole('button', { name: '×' }).click()
  })

  await check('T31', async () => {
    // custom 404 page and its way back. the app is browser-only (ssr: false), so the server
    // answers 200 with the empty page and the 404 is drawn in the browser
    await page.goto(`${BASE}/no-such-page`)
    await page.getByText('Такой строки в ведомости нет').waitFor({ timeout: 10000 })
    await hydrated(page)
    await shot('404')
    await page.getByRole('button', { name: 'На главную' }).click()
    await page.waitForURL('**/admin/overview', { timeout: 10000 })
  })

  await check('T36', async () => {
    // grey skeleton rows while the users list is still loading
    await page.route('**/api/v1/users', async (route) => {
      await new Promise(resolve => setTimeout(resolve, 1500))
      await route.continue()
    })
    await page.goto(`${BASE}/admin/users`)
    await page.locator('.table .skeleton').first().waitFor({ timeout: 5000 })
    await shot('A4-users-loading')
    await rows().filter({ hasText: 'admin' }).first().waitFor({ timeout: 10000 })
    assert(await page.locator('.table .skeleton').count() === 0, 'skeleton left after loading')
    await page.unroute('**/api/v1/users')
  })

  await check('T20', async () => {
    await page.goto(`${BASE}/admin/calendar`)
    await hydrated(page)
    const panel = page.locator('section.day')
    const before = (await panel.innerText()).toLowerCase()
    assert(before.includes('25 октября') && before.includes('воскресенье'), `default panel "${before.slice(0, 60)}"`)
    await shot('U3-calendar')
    await page.locator('.payment', { hasText: 'Дизайн-сервис' }).click()
    const after = (await panel.innerText()).toLowerCase()
    assert(after.includes('26 октября') && after.includes('понедельник') && after.includes('дизайн-сервис'), `after click "${after.slice(0, 80)}"`)
  })

  await check('T47', async () => {
    // calendar month arrows, title dropdown and «Сегодня»
    const title = page.locator('.toolbar .month-picker')
    await title.getByRole('button', { name: 'Следующий месяц' }).click()
    assert((await title.innerText()).replace(/\s+/g, ' ').includes('Ноябрь 2026'), 'arrow did not move to November')
    assert(await page.locator('.payment').count() === 0, 'November should have no payments')
    assert(await page.locator('section.day').count() === 0, 'side panel should close for another month')
    assert((await page.locator('section.month').innerText()).includes('Итоги ноября'), 'totals title not updated')
    await page.getByRole('button', { name: 'Сегодня' }).click()
    assert((await title.innerText()).replace(/\s+/g, ' ').includes('Октябрь 2026'), '«Сегодня» did not come back')
    assert(await page.locator('.payment').count() === 7, 'October payments missing after «Сегодня»')
    await title.locator('.title-label').click()
    await title.locator('.dropdown').getByRole('button', { name: 'Дек' }).click()
    assert((await title.innerText()).replace(/\s+/g, ' ').includes('Декабрь 2026'), 'dropdown pick failed')
    await page.getByRole('button', { name: 'Сегодня' }).click()
  })

  // ---------- users screen (live api) ----------
  await check('T10', async () => {
    await page.goto(`${BASE}/admin/users`)
    await hydrated(page)
    await rows().filter({ hasText: 'admin' }).first().waitFor({ timeout: 10000 })
    await page.locator('aside.panel').waitFor()
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
    await shot('A4-users')
    return `${await rows().count()} users, ${await page.locator('aside.panel .item').count()} persona rows`
  })

  await check('T17', async () => {
    const adminRow = rows().filter({ has: page.locator('.role.admin') })
    assert((await adminRow.innerText()).includes('через .env'), 'no «через .env»')
    assert(await adminRow.getByRole('button', { name: 'Изменить' }).count() === 0, 'admin row is editable')
  })

  await check('T11', async () => {
    await page.getByRole('button', { name: /Добавить персону/ }).click()
    const modal = modalTitled(page, 'Новая персона')
    await modal.getByLabel('Название').fill(P1)
    await modal.getByLabel('Описание').fill('created by ui test')
    await modal.getByRole('button', { name: 'Создать персону' }).click()
    await modal.waitFor({ state: 'detached', timeout: 10000 })
    await page.locator('aside.panel .item', { hasText: P1 }).waitFor({ timeout: 10000 })
  })

  const userModal = modalTitled(page, 'Новый пользователь')

  await check('T12', async () => {
    await page.getByRole('button', { name: 'Новый пользователь' }).click()
    await userModal.waitFor()
    const pass = userModal.locator('input[autocomplete="new-password"]')
    const first = await pass.inputValue()
    assert(first.length === 20, `prefilled password length ${first.length}`)
    await userModal.getByRole('button', { name: 'Сгенерировать' }).click()
    const second = await pass.inputValue()
    assert(second !== first && second.length === 20, 'generate did not change the password')
    assert(await pass.getAttribute('type') === 'password', 'password visible by default')
    await userModal.getByRole('button', { name: 'Показать пароль' }).click()
    assert(await pass.getAttribute('type') === 'text', 'eye did not show the password')
    await userModal.getByRole('button', { name: 'Скрыть пароль' }).click()
    assert(await pass.getAttribute('type') === 'password', 'eye did not hide the password')
    await userModal.locator('.control').click()
    await userModal.locator('.menu .option', { hasText: P1 }).waitFor({ timeout: 5000 })
    await userModal.locator('.control').click()
  })

  await check('T13', async () => {
    await userModal.getByLabel('ФИО').fill('Юай Тестов')
    await userModal.getByLabel('Логин').fill(LOGIN)
    await userModal.locator('.control').click()
    await userModal.getByRole('button', { name: /Новая персона/ }).click()
    const personaModal = modalTitled(page, 'Новая персона')
    await personaModal.waitFor()
    assert(await page.locator('.modal').count() === 2, 'persona modal is not on top of the user form')
    await shot('A4-stacked-modals')
    await personaModal.getByLabel('Название').fill(P2)
    await personaModal.getByRole('button', { name: 'Создать персону' }).click()
    await personaModal.waitFor({ state: 'detached', timeout: 10000 })
    assert(await userModal.getByLabel('ФИО').inputValue() === 'Юай Тестов', 'ФИО lost')
    assert(await userModal.getByLabel('Логин').inputValue() === LOGIN, 'Логин lost')
    if (!(await userModal.locator('.menu').count())) await userModal.locator('.control').click()
    await userModal.locator('.menu .option', { hasText: P2 }).waitFor({ timeout: 5000 })
    await userModal.locator('.menu .option', { hasText: P1 }).click()
  })

  await check('T14', async () => {
    userPassword = await userModal.locator('input[autocomplete="new-password"]').inputValue()
    await userModal.getByRole('button', { name: 'Создать пользователя' }).click()
    await userModal.waitFor({ state: 'detached', timeout: 10000 })
    await rows().filter({ hasText: LOGIN }).waitFor({ timeout: 10000 })
    assert((await rows().filter({ hasText: LOGIN }).innerText()).includes(P1), 'persona not shown in row')
    // same login again must fail inside the form
    await page.getByRole('button', { name: 'Новый пользователь' }).click()
    await userModal.getByLabel('ФИО').fill('Дубль Логина')
    await userModal.getByLabel('Логин').fill(LOGIN)
    await userModal.getByRole('button', { name: 'Создать пользователя' }).click()
    const err = userModal.locator('.error-text')
    await err.waitFor({ timeout: 10000 })
    const text = await err.innerText()
    await userModal.getByRole('button', { name: 'Отмена' }).click()
    return `409 message: "${text}"`
  })

  await check('T15', async () => {
    await rows().filter({ hasText: LOGIN }).getByRole('button', { name: 'Изменить' }).click()
    const modal = modalTitled(page, 'Изменить пользователя')
    await modal.getByLabel('ФИО').fill('Юай Изменённый')
    await modal.getByRole('button', { name: 'Сохранить' }).click()
    await modal.waitFor({ state: 'detached', timeout: 10000 })
    await rows().filter({ hasText: 'Юай Изменённый' }).waitFor({ timeout: 10000 })
  })

  // ---------- regular user ----------
  const userCtx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await userCtx.addInitScript(demoOn)
  const up = await userCtx.newPage()
  watch(up, 'user')

  await check('T7', async () => {
    await up.goto(`${BASE}/login`)
    await hydrated(up)
    await loginWith(up, { login: LOGIN, password: userPassword })
    await up.waitForURL('**/user/overview', { timeout: 15000 })
    const role = await up.locator('.me-role').innerText()
    assert(role === `user · ${P1}`, `role "${role}"`)
    assert(await up.getByRole('button', { name: 'Новый платёж' }).count() === 0, 'user sees «Новый платёж»')
    await up.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
    await up.screenshot({ path: `${SHOTS}U1-overview.png` })
  })

  await check('T8', async () => {
    await up.goto(`${BASE}/admin/users`)
    await up.waitForURL('**/user/overview', { timeout: 15000 })
  })

  await check('T22b', async () => {
    await up.goto(`${BASE}/user/recurring`)
    await hydrated(up)
    assert(await up.locator('.read-only').count() === 1, 'no read-only note')
    assert(await up.getByRole('button', { name: 'Прекратить' }).count() === 0, 'user sees «Прекратить»')
    await up.screenshot({ path: `${SHOTS}U4-recurring.png` })
    await up.goto(`${BASE}/user/payments`)
    await up.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
    await up.screenshot({ path: `${SHOTS}U2-payments.png` })
  })
  await userCtx.close()

  await check('T16', async () => {
    await rows().filter({ hasText: LOGIN }).getByRole('button', { name: 'Изменить' }).click()
    const modal = modalTitled(page, 'Изменить пользователя')
    await modal.getByRole('button', { name: 'Удалить' }).click()
    await modal.getByRole('button', { name: 'Точно удалить?' }).waitFor({ timeout: 5000 })
    assert(await rows().filter({ hasText: LOGIN }).count() === 1, 'deleted after first click')
    await modal.getByRole('button', { name: 'Точно удалить?' }).click()
    await modal.waitFor({ state: 'detached', timeout: 10000 })
    await rows().filter({ hasText: LOGIN }).waitFor({ state: 'detached', timeout: 10000 })
  })

  // ---------- admin pages without figma screens ----------
  await check('T28', async () => {
    const pages = { Журнал: '/admin/log', Настройки: '/admin/settings' }
    for (const [label, path] of Object.entries(pages)) {
      await page.locator('aside.sidebar').getByRole('link', { name: label }).click()
      await page.waitForURL(`**${path}`, { timeout: 10000 })
      // the new page fades in after the url changes (pageTransition), wait for its title
      await page.locator('h1.title', { hasText: label }).waitFor({ timeout: 5000 }).catch(() => {})
      const title = await page.locator('h1.title').innerText()
      assert(title === label, `${path}: title "${title}"`)
      assert(await page.locator('aside.sidebar .item.active').innerText() === label, `${path}: nav item not active`)
    }
    // personas live in the panel on the users page now, the page and its nav item are gone
    assert(await page.locator('aside.sidebar').getByRole('link', { name: 'Персоны' }).count() === 0, 'Персоны still in the nav')
  })

  await check('T29', async () => {
    const P3 = `uitest_p3_${tag}`
    // personas are managed from the panel on the users page
    await page.goto(`${BASE}/admin/users`)
    await hydrated(page)
    const items = () => page.locator('.panel .item')
    await page.getByRole('button', { name: /Добавить персону/ }).click()
    let modal = modalTitled(page, 'Новая персона')
    await modal.getByLabel('Название').fill(P3)
    await modal.getByLabel('Описание').fill('first')
    await modal.getByRole('button', { name: 'Создать персону' }).click()
    await modal.waitFor({ state: 'detached', timeout: 10000 })
    await items().filter({ hasText: P3 }).waitFor({ timeout: 10000 })

    // edit: rename and clear the description
    await items().filter({ hasText: P3 }).getByRole('button', { name: 'Изменить' }).click()
    modal = modalTitled(page, 'Изменить персону')
    await modal.getByLabel('Название').fill(`${P3}_v2`)
    await modal.getByLabel('Описание').fill('')
    await modal.getByRole('button', { name: 'Сохранить' }).click()
    await modal.waitFor({ state: 'detached', timeout: 10000 })
    const renamed = items().filter({ hasText: `${P3}_v2` })
    await renamed.waitFor({ timeout: 10000 })
    assert(await renamed.locator('.description').count() === 0, 'description not cleared')
    await page.screenshot({ path: `${SHOTS}A5-personas.png` })

    // delete: first click asks, second deletes
    await renamed.getByRole('button', { name: 'Изменить' }).click()
    await modal.getByRole('button', { name: 'Удалить' }).click()
    await modal.getByRole('button', { name: 'Точно удалить?' }).click()
    await modal.waitFor({ state: 'detached', timeout: 10000 })
    await renamed.waitFor({ state: 'detached', timeout: 10000 })
  })

  await check('T30', async () => {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
    const p = await ctx.newPage()
    await p.goto(`${BASE}/login`)
    await hydrated(p)
    await loginWith(p, ADMIN)
    await p.waitForURL('**/admin/overview', { timeout: 15000 })
    await p.getByRole('button', { name: 'Выйти' }).click()
    await p.waitForURL('**/login', { timeout: 10000 })
    await p.goto(`${BASE}/admin/overview`)
    await p.waitForURL('**/login', { timeout: 10000 })
    await ctx.close()
  })

  // a fresh browser with the admin's cookies, so settings saved here don't leak into other checks
  const prefsContext = () => browser.newContext({ viewport: { width: 1920, height: 1080 }, reducedMotion: 'reduce', storageState: { cookies: [], origins: [] } })
    .then(async (ctx) => {
      await ctx.addCookies((await admin.storageState()).cookies)
      return ctx
    })

  await check('T48', async () => {
    // live mode (demo off) talks to the payment api. the server has no hard delete for payments,
    // so instead of writing to it the payment endpoints answer from here and every write is recorded
    const ctx = await prefsContext()
    const p = await ctx.newPage()
    watch(p, 'live')
    const today = new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Moscow' }).format(new Date())
    const shift = (n) => {
      const d = new Date(`${today}T12:00:00`)
      d.setDate(d.getDate() + n)
      return d.toISOString().slice(0, 10)
    }
    const recurring = [
      { id: 'r1', amount: 12000000, currency: 'RUB', description: 'Аренда офиса', frequency: 'monthly', interval: 1, start_date: shift(-6), assignee_name: 'Ольга Иванова' },
      { id: 'r2', amount: 4990, currency: 'USD', description: 'Сервер VPS', frequency: 'monthly', interval: 1, start_date: shift(1), assignee_name: 'Дмитрий Ким' },
    ]
    const occurrences = [
      { id: 'o1', recurrent_payment_id: 'r1', amount: 12000000, currency: 'RUB', description: 'Аренда офиса', due_date: shift(-6), status: 'overdue', assignee_name: 'Ольга Иванова' },
      { id: 'o2', recurrent_payment_id: 'r2', amount: 4990, currency: 'USD', description: 'Сервер VPS', due_date: shift(1), status: 'pending', assignee_name: 'Дмитрий Ким' },
    ]
    const writes = []
    await p.route('**/api/v1/payment-occurrences**', (route) => {
      const req = route.request()
      if (req.method() === 'GET') return route.fulfill({ json: { items: occurrences, total: occurrences.length } })
      writes.push(`${req.method()} ${new URL(req.url()).pathname} ${req.postData()}`)
      const item = occurrences.find(o => req.url().endsWith(o.id))
      Object.assign(item, { status: 'paid', paid_date: today })
      return route.fulfill({ status: 204 })
    })
    await p.route('**/api/v1/recurring-payments**', (route) => {
      const req = route.request()
      if (req.method() === 'GET') return route.fulfill({ json: recurring })
      writes.push(`${req.method()} ${new URL(req.url()).pathname} ${req.postData() ?? ''}`)
      if (req.method() === 'DELETE') recurring.find(r => req.url().endsWith(r.id)).stopped_at = new Date().toISOString()
      return route.fulfill(req.method() === 'POST' ? { status: 201, json: { id: 'r9' } } : { status: 204 })
    })

    await p.goto(`${BASE}/admin/overview`)
    await hydrated(p)
    assert(await p.locator('.soon').count() === 0, '«скоро» left on Обзор')
    await p.locator('.banner').getByRole('button', { name: 'Отметить оплату' }).click()
    await p.locator('.banner').waitFor({ state: 'detached', timeout: 5000 })
    assert(writes[0] === `PATCH /api/v1/payment-occurrences/o1 {"action":"confirm","paid_date":"${today}"}`, `mark: ${writes[0]}`)

    await p.goto(`${BASE}/admin/recurring`)
    await hydrated(p)
    const vps = rowsOf(p).filter({ hasText: 'Сервер VPS' })
    await vps.getByRole('button', { name: 'Прекратить' }).click()
    assert(writes.length === 1, 'stopped on the first click')
    await vps.getByRole('button', { name: 'Точно прекратить?' }).click()
    await vps.waitFor({ state: 'detached', timeout: 5000 })
    assert(writes[1] === 'DELETE /api/v1/recurring-payments/r2 ', `stop: ${writes[1]}`)

    await p.getByRole('button', { name: 'Новый платёж' }).click()
    const modal = modalTitled(p, 'Новый регулярный платёж')
    await modal.getByLabel('Название').fill('Аренда склада')
    await modal.getByLabel('Сумма').fill('45000,50')
    await modal.getByLabel('Ответственный').click()
    await modal.locator('.menu .option').first().click()
    await modal.getByRole('button', { name: 'Добавить платёж' }).click()
    await modal.waitFor({ state: 'detached', timeout: 5000 })
    const body = JSON.parse(writes[2]?.split(' ').slice(2).join(' ') ?? '{}')
    assert(body.amount === 4500050 && body.description === 'Аренда склада' && body.frequency === 'monthly' && body.interval === 1 && body.assignee_id,
      `create: ${writes[2]}`)
    await ctx.close()
    return `${writes.length} writes, all as in Swagger`
  })

  await check('T49', async () => {
    // theme switch in the sidebar, accessibility switches in Настройки, all kept after reload
    const ctx = await prefsContext()
    const p = await ctx.newPage()
    watch(p, 'prefs')
    await p.goto(`${BASE}/admin/overview`)
    await hydrated(p)
    const attr = name => p.evaluate(n => document.documentElement.dataset[n] ?? '', name)
    assert(await attr('theme') === 'dark', 'not dark by default')
    await p.getByRole('button', { name: 'Светлая тема' }).click()
    assert(await attr('theme') === 'light', 'theme button did nothing')
    const bg = await p.evaluate(() => getComputedStyle(document.body).backgroundColor)
    await p.goto(`${BASE}/admin/settings`)
    await hydrated(p)
    await p.getByRole('button', { name: /Доступность/ }).click()
    await p.getByRole('switch', { name: 'Меньше движения' }).click()
    await p.getByRole('switch', { name: 'Шрифт для дислексии' }).click()
    await p.reload()
    await hydrated(p)
    assert(await attr('theme') === 'light', 'theme lost after reload')
    assert(await attr('motion') === 'reduced', 'less motion lost after reload')
    assert(await attr('font') === 'readable', 'readable font lost after reload')
    assert(await p.locator('link#readable-font').count() === 1, 'readable font not loaded')
    await p.screenshot({ path: `${SHOTS}A8-settings-light.png` })
    await ctx.close()
    return `light body bg ${bg}`
  })

  await check('T50', async () => {
    // demo mode: the actions change the demo data in this browser only
    const ctx = await prefsContext()
    const p = await ctx.newPage()
    watch(p, 'demo')
    await p.goto(`${BASE}/admin/settings`)
    await hydrated(p)
    await p.getByRole('button', { name: /Демо-режим/ }).click()
    await p.getByRole('switch', { name: 'Демо-режим' }).click()
    await p.locator('aside .demo').waitFor()

    // Обзор: paying the overdue rent removes the banner
    await p.locator('aside.sidebar').getByRole('link', { name: 'Обзор' }).click()
    await p.waitForURL('**/admin/overview')
    await p.getByRole('button', { name: 'Отметить оплату' }).click()
    await p.locator('.banner').waitFor({ state: 'detached', timeout: 5000 })

    // Платежи: a paid row opens the popover too, and the mark can be taken off
    await p.locator('aside.sidebar').getByRole('link', { name: 'Платежи' }).click()
    await p.waitForURL('**/admin/payments')
    await rowsOf(p).filter({ hasText: 'Аренда офиса' }).click()
    await p.getByRole('button', { name: 'Снять отметку' }).click()
    await rowsOf(p).filter({ hasText: 'Аренда офиса' }).filter({ hasText: /Просроч/ }).waitFor({ timeout: 5000 })

    // Регулярные: stop one, add one
    await p.locator('aside.sidebar').getByRole('link', { name: 'Регулярные' }).click()
    await p.waitForURL('**/admin/recurring')
    const before = await rowsOf(p).count()
    await rowsOf(p).filter({ hasText: 'Почта Workspace' }).getByRole('button', { name: 'Прекратить' }).click()
    await rowsOf(p).filter({ hasText: 'Почта Workspace' }).getByRole('button', { name: 'Точно прекратить?' }).click()
    await p.waitForFunction(n => document.querySelectorAll('.table .row:not(.head)').length === n - 1, before)
    await p.getByRole('button', { name: 'Новый платёж' }).click()
    const modal = modalTitled(p, 'Новый регулярный платёж')
    await modal.getByRole('button', { name: 'Добавить платёж' }).click()
    await modal.getByText('Введите название').waitFor()
    await modal.getByLabel('Название').fill('Тестовый склад')
    await modal.getByLabel('Сумма').fill('45000')
    await modal.getByLabel('Ответственный').click()
    await modal.locator('.menu .option').first().click()
    await modal.getByRole('button', { name: 'Добавить платёж' }).click()
    await modal.waitFor({ state: 'detached' })
    await rowsOf(p).filter({ hasText: 'Тестовый склад' }).waitFor()

    // reset brings the seed data back
    await p.locator('aside.sidebar').getByRole('link', { name: 'Настройки' }).click()
    await p.waitForURL('**/admin/settings')
    await p.getByRole('button', { name: /Демо-режим/ }).click()
    await p.getByRole('button', { name: 'Сбросить', exact: true }).click()
    await p.locator('aside.sidebar').getByRole('link', { name: 'Регулярные' }).click()
    await p.waitForURL('**/admin/recurring')
    assert(await rowsOf(p).filter({ hasText: 'Тестовый склад' }).count() === 0, 'reset kept the added payment')
    await ctx.close()
  })

  await check('T51', async () => {
    // chat under the ask bar: grows with the answer, fits the screen, turns into a button on click outside
    const ctx = await prefsContext()
    const p = await ctx.newPage()
    watch(p, 'chat')
    await p.goto(`${BASE}/admin/overview`)
    await hydrated(p)
    const bar = p.getByLabel('Вопрос ассистенту')

    // the live assistant answers or says it's unavailable, either way in words
    await bar.fill('Что просрочено?')
    await bar.press('Enter')
    await p.waitForFunction(() => document.querySelector('.chat .answer')?.textContent?.trim(), null, { timeout: 30000 })
    const live = await p.locator('.chat .answer').last().innerText()

    // demo answers come from the demo data
    await p.evaluate(() => {
      const prefs = JSON.parse(localStorage.getItem('invoice-keeper:prefs') ?? '{}')
      localStorage.setItem('invoice-keeper:prefs', JSON.stringify({ ...prefs, demoMode: true }))
    })
    await p.reload()
    await hydrated(p)
    await bar.fill('Кто отвечает за аренду?')
    await bar.press('Enter')
    await p.locator('.chat .answer', { hasText: 'Ольга Иванова' }).waitFor({ timeout: 5000 })
    const questions = ['Что просрочено?', 'Сколько платим в рублях?', 'Что оплатить на этой неделе?', 'Сколько платим в долларах в этом месяце?']
    // twice, so the chat gets longer than the screen
    for (const q of [...questions, ...questions]) {
      await bar.fill(q)
      await bar.press('Enter')
      await p.waitForFunction(() => !document.querySelector('.chat .pending, .chat .typing'))
    }
    const box = await p.locator('.chat').boundingBox()
    assert(box.width > 300 && box.width <= 1920 / 4 + 2, `chat width ${box.width}`)
    assert(box.y + box.height <= 1080, 'chat goes below the screen')
    assert(await p.locator('.chat .thumb').isVisible(), 'no scrollbar on a long chat')

    await p.mouse.click(700, 900)
    const resume = p.getByRole('button', { name: /Чат с ассистентом/ })
    await resume.waitFor({ timeout: 3000 })
    await resume.click()
    assert(await p.locator('.chat .mine').count() === 9, 'chat history lost after reopening')
    await p.screenshot({ path: `${SHOTS}A1-chat.png` })
    await ctx.close()
    return `live: ${live.slice(0, 60)}`
  })

  await check('T44', async () => {
    // with motion on, filters and the calendar go through view transitions and still end up right
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, storageState: await admin.storageState() })
    await ctx.addInitScript(() => {
      window.__transitions = 0
      const original = document.startViewTransition?.bind(document)
      if (original) document.startViewTransition = (cb) => { window.__transitions++; return original(cb) }
    })
    const p = await ctx.newPage()
    await p.goto(`${BASE}/admin/payments`)
    await hydrated(p)
    await p.locator('.filters .seg .item', { hasText: 'Просрочено' }).click()
    await p.waitForFunction(() => document.querySelectorAll('.table .row:not(.head)').length === 1, null, { timeout: 5000 })
    await p.locator('.filters .seg .item', { hasText: 'Все' }).click()
    await p.waitForFunction(() => document.querySelectorAll('.table .row:not(.head)').length === 7, null, { timeout: 5000 })
    // go through the sidebar, a full reload would reset the counter
    await p.locator('aside.sidebar').getByRole('link', { name: 'Календарь' }).click()
    await p.waitForURL('**/admin/calendar')
    await p.locator('.payment', { hasText: 'Дизайн-сервис' }).click()
    await p.waitForFunction(() => document.querySelector('section.day')?.textContent?.includes('26 октября'), null, { timeout: 5000 })
    const count = await p.evaluate(() => window.__transitions)
    await ctx.close()
    assert(count >= 3, `only ${count} view transitions ran`)
    return `${count} view transitions`
  })
}
finally {
  // ---------- cleanup, runs even if a step failed ----------
  await check('T26', async () => {
    const api = admin.request
    const users = await (await api.get(`${BASE}/api/v1/users`)).json()
    for (const u of users.items.filter(u => u.login.startsWith('uitest_'))) await api.delete(`${BASE}/api/v1/users/${u.id}`)
    const personas = await (await api.get(`${BASE}/api/v1/personas`)).json()
    for (const p of personas.items.filter(p => p.name.startsWith('uitest_'))) await api.delete(`${BASE}/api/v1/personas/${p.id}`)
    const usersLeft = (await (await api.get(`${BASE}/api/v1/users`)).json()).items.filter(u => u.login.startsWith('uitest_')).length
    const personasLeft = (await (await api.get(`${BASE}/api/v1/personas`)).json()).items.filter(p => p.name.startsWith('uitest_')).length
    assert(usersLeft === 0 && personasLeft === 0, `left: ${usersLeft} users, ${personasLeft} personas`)
  })
  await browser.close()
}

// the browser logs every failed request. these are expected: 401 from the guest check before login,
// 409 from the duplicate-login test, 503 when the live assistant is down
const expected = /Failed to load resource: the server responded with a status of (401|409|503)/
const realIssues = consoleIssues.filter(issue => !expected.test(issue))
results.T23 = { ok: realIssues.length === 0, note: `${realIssues.length} console issues (${consoleIssues.length - realIssues.length} expected request errors)` }
writeFileSync(new URL('./results.json', import.meta.url), JSON.stringify({ results, consoleIssues }, null, 2))
console.log('\nCONSOLE ISSUES:\n' + (consoleIssues.join('\n') || 'none'))
process.exitCode = Object.values(results).some(r => !r.ok) ? 1 : 0
