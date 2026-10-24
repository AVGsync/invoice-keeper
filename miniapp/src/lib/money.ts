import type { Currency } from '../api/types'

// Форматирование денег. Суммы — целое число минимальных единиц (копейки/центы).
// Свой форматтер (не Intl), чтобы вывод был одинаков во всех движках.

const NNBSP = ' ' // narrow no-break space — группировка разрядов
const NBSP = ' ' // no-break space — перед символом валюты
const MINUS = '−' // настоящий минус

const SYMBOL: Record<Currency, string> = {
  RUB: '₽',
  USD: '$',
  EUR: '€',
}

function groupInteger(digits: string): string {
  let out = ''
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += NNBSP
    out += digits[i]
  }
  return out
}

export function formatMoney(minor: number, cur: Currency): string {
  const negative = minor < 0
  const abs = Math.abs(minor)
  const major = Math.floor(abs / 100)
  const frac = abs % 100

  let num = groupInteger(String(major))
  if (frac !== 0) {
    num += ',' + String(frac).padStart(2, '0')
  }

  let result = num + NBSP + SYMBOL[cur]
  if (negative) result = MINUS + result
  return result
}

// Разбор пользовательского ввода суммы в минорные единицы.
// Принимает "," или ".", ≤2 знака дроби, только цифры. Результат > 0 и ≤ 1e13.
export function parseMoneyInput(s: string): number | null {
  if (typeof s !== 'string') return null
  let t = s.replace(/[\s  ]/g, '')
  if (t === '') return null
  t = t.replace(',', '.')
  if (!/^\d+(\.\d{1,2})?$/.test(t)) return null

  const dot = t.indexOf('.')
  let major: string
  let frac: string
  if (dot === -1) {
    major = t
    frac = '00'
  } else {
    major = t.slice(0, dot)
    frac = t.slice(dot + 1).padEnd(2, '0')
  }
  const minor = Number(major) * 100 + Number(frac)
  if (!Number.isFinite(minor) || minor <= 0 || minor > 1e13) return null
  return minor
}

const ORDER: Currency[] = ['RUB', 'USD', 'EUR']

// Суммирование по валютам. Валюты НИКОГДА не складываются между собой.
export function sumByCurrency<T>(
  items: readonly T[],
  pick: (item: T) => { currency: Currency; amount: number } | null,
): Record<Currency, number> {
  const acc: Record<Currency, number> = { RUB: 0, USD: 0, EUR: 0 }
  for (const item of items) {
    const v = pick(item)
    if (!v) continue
    acc[v.currency] += v.amount
  }
  return acc
}

// Порядок и фильтрация ненулевых для UI.
export function nonZeroCurrencies(totals: Record<Currency, number>): Currency[] {
  return ORDER.filter((c) => totals[c] !== 0)
}

export { ORDER as CURRENCY_ORDER }
