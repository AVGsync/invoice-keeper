// what the screens show for a payment period and a recurring payment, made from the server's data
// in utils/payments.ts, and the per-currency totals
import { fromRu } from './date'
import { formatAmount, formatMoney, parseMoney, plural } from './money'

// ended: not paid, and its recurring payment was stopped (not a debt any more)
export type PeriodStatus = 'pending' | 'paid' | 'late' | 'overdue' | 'ended'
export type Currency = 'RUB' | 'USD' | 'EUR'

export interface Person {
  initials: string
  short: string
  name: string
  persona: string
}

export interface Period {
  id: string
  // who has to pay it and which recurring payment it came from
  userId?: string
  templateId?: string
  title: string
  frequency: string
  meta: string
  who: Person
  due: string
  relative?: string
  amount: string
  // the same amount in kopecks / cents, for sums
  amountMinor?: number
  currency: Currency
  status: PeriodStatus
  paidAt?: string
  overdue?: string
  // date the telegram reminder went out
  reminded?: string
}

export interface CurrencyTotal {
  code: Currency
  name: string
  count: string
  total: string
  paid: string
  unpaid: string
  overdue?: string
  // share of the month already paid, 0-1
  progress: number
}

const currencyNames: Record<Currency, string> = { RUB: 'Российский рубль', USD: 'Доллар США', EUR: 'Евро' }

// totals per currency on a date: what was due by then, plus what got paid early.
// counted here: GET /payments/summary filters by due date only,
// so it can't count early payments
export function totalsOn(dateIso: string, list: Period[]): CurrencyTotal[] {
  const paidBy = (period: Period) => !!period.paidAt && fromRu(period.paidAt) <= dateIso
  const sum = (list: Period[]) => list.reduce((total, period) => total + parseMoney(period.amount), 0)

  return (Object.keys(currencyNames) as Currency[]).map((code) => {
    // an unpaid period of a stopped payment isn't owed any more, the server leaves it out too
    const counted = list.filter(p => p.currency === code && p.status !== 'ended' && (fromRu(p.due) <= dateIso || paidBy(p)))
    const paid = counted.filter(paidBy)
    const unpaid = counted.filter(p => !paidBy(p))
    const overdue = unpaid.filter(p => fromRu(p.due) < dateIso)
    const total = sum(counted)

    return {
      code,
      name: currencyNames[code],
      count: `${counted.length} ${plural(counted.length, ['платёж', 'платежа', 'платежей'])}`,
      total: formatMoney(total, code),
      paid: formatAmount(sum(paid)),
      unpaid: formatAmount(sum(unpaid)),
      overdue: overdue.length ? formatAmount(sum(overdue)) : undefined,
      progress: total ? sum(paid) / total : 0,
    }
  })
}

export interface Template {
  id: string
  assigneeId?: string
  title: string
  category: string
  amount: string
  currency: Currency
  frequency: string
  day: string
  who: Person
  nextDue?: string
  ended?: boolean
}

// the search pills above «Периоды», '' = off.
// amounts are typed in rubles / dollars / euros, dates are YYYY-MM-DD
export interface PeriodColumnFilters {
  title: string
  dueFrom: string
  dueTo: string
  amountFrom: string
  amountTo: string
  paidFrom: string
  paidTo: string
}

export const noPeriodColumnFilters = (): PeriodColumnFilters =>
  ({ title: '', dueFrom: '', dueTo: '', amountFrom: '', amountTo: '', paidFrom: '', paidTo: '' })

// the search pills above «Регулярные», '' = off. frequency is the server's: weekly | monthly
export interface TemplateColumnFilters {
  title: string
  amountFrom: string
  amountTo: string
  frequency: string
  nextFrom: string
  nextTo: string
}

export const noTemplateColumnFilters = (): TemplateColumnFilters =>
  ({ title: '', amountFrom: '', amountTo: '', frequency: '', nextFrom: '', nextTo: '' })
