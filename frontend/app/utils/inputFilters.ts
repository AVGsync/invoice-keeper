import { formatAmount, parseMoney } from './money'

// used with UiInput's `filter` (runs while typing) and `format` (runs when the field loses focus)

// digits and one decimal comma with at most 2 digits after it. a dot becomes a comma
export function moneyFilter(text: string): string {
  const clean = text.replace(/\./g, ',').replace(/[^\d,]/g, '')
  const [whole = '', ...rest] = clean.split(',')
  return rest.length ? `${whole},${rest.join('').slice(0, 2)}` : whole
}

// '45000' -> '45 000,00'
export function moneyFormat(text: string): string {
  return text ? formatAmount(parseMoney(text)) : ''
}

// what the backend accepts for a login: 3-32 of a-z A-Z 0-9 _ . -
export function loginFilter(text: string): string {
  return text.replace(/[^\w.-]/g, '').slice(0, 32)
}
