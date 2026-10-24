import type { Currency } from '~/utils/periods'

// '120 000,00 ₽' -> 120000
export function parseMoney(text: string): number {
  return Number(text.replace(/[^\d,.-]/g, '').replace(',', '.')) || 0
}

// 45000 -> '45 000,00'
export function formatAmount(value: number): string {
  return value.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

// 45000, 'RUB' -> '45 000,00 ₽'
export function formatMoney(value: number, currency: Currency): string {
  return value.toLocaleString('ru-RU', { style: 'currency', currency })
}

// 1 платёж, 3 платежа, 5 платежей
export function plural(n: number, [one, few, many]: [string, string, string]): string {
  const tens = n % 100
  const last = n % 10
  if (tens >= 11 && tens <= 14) return many
  if (last === 1) return one
  if (last >= 2 && last <= 4) return few
  return many
}
