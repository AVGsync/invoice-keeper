// lists every piece of text with contrast under 3:1 on each screen and modal (logs in as admin).
// the login receipt is skipped, it has its own paper colours.
//   npm run test:contrast
// env: UI_BASE, UI_ADMIN_LOGIN, UI_ADMIN_PASSWORD, same as run.mjs; UI_THEME=light checks the light theme
import { chromium } from 'playwright'

const BASE = process.env.UI_BASE ?? 'http://localhost:3000'
const ADMIN = { login: process.env.UI_ADMIN_LOGIN ?? 'admin', password: process.env.UI_ADMIN_PASSWORD ?? 'admin123' }
let problems = 0
const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
const theme = process.env.UI_THEME ?? 'dark'
await page.addInitScript(t => localStorage.setItem('invoice-keeper:prefs', JSON.stringify({ theme: t })), theme)
console.log(`theme: ${theme}`)
const hydrated = () => page.waitForFunction(() => !!document.querySelector('#__nuxt')?.__vue_app__).then(() => page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {}))

// every visible text element whose contrast with the real background behind it is under 3:1
function audit() {
  const parse = (c) => {
    const m = c.match(/[\d.]+/g)?.map(Number) ?? [0, 0, 0, 0]
    // color-mix() comes back as color(srgb 0.9 0.9 0.9 / 0.5), channels from 0 to 1
    const scale = c.startsWith('color(') ? 255 : 1
    return { r: m[0] * scale, g: m[1] * scale, b: m[2] * scale, a: m[3] ?? 1 }
  }
  const lum = ({ r, g, b }) => {
    const f = v => (v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
  }
  const blend = (top, bottom) => ({
    r: top.r * top.a + bottom.r * (1 - top.a),
    g: top.g * top.a + bottom.g * (1 - top.a),
    b: top.b * top.a + bottom.b * (1 - top.a),
    a: 1,
  })
  const bgOf = (el) => {
    // the picked segment sits on a sliding thumb next to it, not inside it
    const thumb = el.closest('.seg .item.active')?.parentElement.querySelector('.thumb')
    if (thumb) el = thumb
    const layers = []
    for (let n = el; n; n = n.parentElement) {
      const c = parse(getComputedStyle(n).backgroundColor)
      if (c.a > 0) layers.push(c)
      if (c.a === 1) break
    }
    return layers.reverse().reduce((acc, c) => blend(c, acc), { r: 255, g: 255, b: 255, a: 1 })
  }
  const out = []
  for (const el of document.querySelectorAll('body *')) {
    const own = [...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim())
    if (!own) continue
    const st = getComputedStyle(el)
    if (st.visibility === 'hidden' || st.display === 'none' || el.closest('.receipt, nuxt-devtools-frame, #nuxt-devtools-container')) continue
    const box = el.getBoundingClientRect()
    if (!box.width || !box.height) continue
    const bg = bgOf(el)
    const fg = blend(parse(st.color), bg)
    const [a, b] = [lum(fg), lum(bg)].sort((x, y) => y - x)
    const ratio = (a + 0.05) / (b + 0.05)
    if (ratio < 3) out.push(`${ratio.toFixed(2)} | ${el.tagName.toLowerCase()}.${[...el.classList].join('.')} | "${el.textContent.trim().slice(0, 30)}" | color ${st.color} on ${Math.round(bg.r)},${Math.round(bg.g)},${Math.round(bg.b)}`)
  }
  return out
}

async function report(name) {
  const rows = await page.evaluate(audit)
  problems += rows.length
  console.log(`\n### ${name} (${rows.length})`)
  for (const r of [...new Set(rows)]) console.log('  ' + r)
}

await page.goto(`${BASE}/login`)
await hydrated()
await report('login')
await page.locator('form input').first().fill(ADMIN.login)
await page.locator('form input[type="password"]').fill(ADMIN.password)
await page.getByRole('button', { name: 'Войти' }).click()
await page.waitForURL('**/admin/overview')
await hydrated()
await report('overview')
for (const path of ['payments', 'calendar', 'recurring', 'settings', 'users']) {
  await page.goto(`${BASE}/admin/${path}`)
  await hydrated()
  await report(path)
}
await page.getByRole('button', { name: 'Новый пользователь' }).click()
await page.locator('.modal').waitFor()
await page.locator('.modal .control').click()
await report('users + new user modal')
await page.goto(`${BASE}/admin/payments`)
await hydrated()
await page.locator('.table .row:not(.head)', { hasText: 'Сервер VPS' }).click()
await report('payments + popover')
await page.goto(`${BASE}/admin/recurring`)
await hydrated()
await page.getByRole('button', { name: 'Новый платёж' }).click()
await report('recurring + new payment modal')
await browser.close()
process.exitCode = problems ? 1 : 0
