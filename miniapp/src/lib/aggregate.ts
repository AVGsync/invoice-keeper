import type { Currency, PaymentResponse } from '../api/types'
import { addDays, diffDays, monthKey, weekStart, weekLabel, monthRange } from './dates'
import { sumByCurrency } from './money'

export interface CurrencyMonthTotals {
  paid: number
  outstanding: number
  overdue: number
}

// Суммы по месяцу, по правилам /payments/summary:
// paid → paid; overdue → outstanding + overdue; pending → outstanding;
// ended без оплаты → никуда.
export function monthTotals(
  items: readonly PaymentResponse[],
  ym: string,
): Partial<Record<Currency, CurrencyMonthTotals>> {
  const { from, to } = monthRange(ym)
  const out: Partial<Record<Currency, CurrencyMonthTotals>> = {}
  const ensure = (c: Currency): CurrencyMonthTotals => {
    let v = out[c]
    if (!v) {
      v = { paid: 0, outstanding: 0, overdue: 0 }
      out[c] = v
    }
    return v
  }
  for (const it of items) {
    if (it.due_date < from || it.due_date > to) continue
    const t = ensure(it.currency)
    switch (it.status) {
      case 'paid':
        t.paid += it.amount
        break
      case 'overdue':
        t.outstanding += it.amount
        t.overdue += it.amount
        break
      case 'pending':
        t.outstanding += it.amount
        break
      case 'ended':
        break
    }
  }
  return out
}

export interface WeekGroup {
  weekStart: string
  label: string
  items: PaymentResponse[]
  totals: Record<Currency, number>
}

export function groupByWeek(items: readonly PaymentResponse[], todayStr: string): WeekGroup[] {
  const map = new Map<string, PaymentResponse[]>()
  for (const it of items) {
    const ws = weekStart(it.due_date)
    const arr = map.get(ws)
    if (arr) arr.push(it)
    else map.set(ws, [it])
  }
  const starts = [...map.keys()].sort()
  return starts.map((ws) => {
    const list = map.get(ws)!
    return {
      weekStart: ws,
      label: weekLabel(ws, todayStr),
      items: list,
      totals: sumByCurrency(list, (i) => ({ currency: i.currency, amount: i.amount })),
    }
  })
}

// pending с due_date в [today, today+days-1].
export function upcoming(
  items: readonly PaymentResponse[],
  todayStr: string,
  days = 7,
): PaymentResponse[] {
  const end = addDays(todayStr, days - 1)
  return items
    .filter((i) => i.status === 'pending' && i.due_date >= todayStr && i.due_date <= end)
    .sort((a, b) => (a.due_date < b.due_date ? -1 : a.due_date > b.due_date ? 1 : 0))
}

export function overdue(items: readonly PaymentResponse[]): PaymentResponse[] {
  return items
    .filter((i) => i.status === 'overdue')
    .sort((a, b) => (a.due_date < b.due_date ? -1 : a.due_date > b.due_date ? 1 : 0))
}

export { monthKey, diffDays }
