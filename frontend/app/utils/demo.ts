// demo data (from the original design), shown when demo mode is on in Настройки.
// screens read payments through usePayments(), which picks this or the server's data
import { fromRu } from './date'
import { formatAmount, formatMoney, parseMoney, plural } from './money'

// on the demo screens "today" is 24.10.2026
export const demoToday = '2026-10-24'

// ended: not paid, and its recurring payment was stopped (not a debt any more)
export type PeriodStatus = 'pending' | 'paid' | 'late' | 'overdue' | 'ended'
export type Currency = 'RUB' | 'USD' | 'EUR'

export interface Person {
  initials: string
  short: string
  name: string
  persona: string
}

export const people = {
  olga: { initials: 'ОИ', short: 'О. Иванова', name: 'Ольга Иванова', persona: 'Бухгалтер' },
  dima: { initials: 'ДК', short: 'Д. Ким', name: 'Дмитрий Ким', persona: 'DevOps' },
  anna: { initials: 'АС', short: 'А. Смирнова', name: 'Анна Смирнова', persona: 'Руководитель' },
} satisfies Record<string, Person>

// who is logged in on the user screens
export const currentUser = people.olga

export interface Period {
  id: string
  // who has to pay it and which recurring payment it came from (live data only)
  userId?: string
  templateId?: string
  title: string
  frequency: string
  meta: string
  who: Person
  due: string
  relative?: string
  amount: string
  // the same amount in kopecks / cents, for sums (live data only)
  amountMinor?: number
  currency: Currency
  status: PeriodStatus
  paidAt?: string
  overdue?: string
  // date the telegram reminder went out
  reminded?: string
}

export const seedPeriods: Period[] = [
  { id: 'demo-p1', title: 'Интернет в офисе', frequency: 'ежемесячно', meta: 'ежемесячно · офис', who: people.olga, due: '05.10.2026', amount: '1 500,00 ₽', currency: 'RUB', status: 'paid', paidAt: '05.10.2026' },
  { id: 'demo-p2', title: 'Аренда офиса', frequency: 'ежемесячно', meta: 'ежемесячно · офис', who: people.olga, due: '18.10.2026', relative: '6 дн. назад', amount: '120 000,00 ₽', currency: 'RUB', status: 'overdue', overdue: '6 дн.' },
  { id: 'demo-p3', title: 'Хостинг бэкапов', frequency: 'ежемесячно', meta: 'ежемесячно · хостинг', who: people.dima, due: '20.10.2026', amount: '12,00 $', currency: 'USD', status: 'late', paidAt: '22.10.2026', overdue: '+2 дн.' },
  { id: 'demo-p4', title: 'Сервер VPS', frequency: 'ежемесячно', meta: 'ежемесячно · хостинг', who: people.dima, due: '25.10.2026', relative: 'завтра', amount: '49,90 $', currency: 'USD', status: 'pending', reminded: '22.10' },
  { id: 'demo-p5', title: 'Дизайн-сервис', frequency: 'ежемесячно', meta: 'ежемесячно · подписка', who: people.anna, due: '26.10.2026', relative: 'через 2 дн.', amount: '15,00 €', currency: 'EUR', status: 'paid', paidAt: '20.10.2026' },
  { id: 'demo-p6', title: 'Почта Workspace', frequency: 'ежемесячно', meta: 'ежемесячно · подписка', who: people.anna, due: '30.10.2026', relative: 'через 6 дн.', amount: '36,00 €', currency: 'EUR', status: 'pending' },
  { id: 'demo-p7', title: 'Домен invoice-keeper.ru', frequency: 'ежегодно', meta: 'ежегодно · инфраструктура', who: people.dima, due: '31.10.2026', relative: 'через 7 дн.', amount: '1 990,00 ₽', currency: 'RUB', status: 'pending' },
]

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
// counted here for both demo and live data: GET /payments/summary filters by due date only,
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
  // live data only
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

export const seedTemplates: Template[] = [
  { id: 'demo-t1', title: 'Аренда офиса', category: 'Офис', amount: '120 000,00 ₽', currency: 'RUB', frequency: 'ежемесячно', day: '18-е число', who: people.olga, nextDue: '18.11.2026' },
  { id: 'demo-t2', title: 'Интернет в офисе', category: 'Офис', amount: '1 500,00 ₽', currency: 'RUB', frequency: 'ежемесячно', day: '5-е число', who: people.olga, nextDue: '05.11.2026' },
  { id: 'demo-t3', title: 'Домен invoice-keeper.ru', category: 'Инфраструктура', amount: '1 990,00 ₽', currency: 'RUB', frequency: 'ежегодно', day: '31 октября', who: people.dima, nextDue: '31.10.2026' },
  { id: 'demo-t4', title: 'Сервер VPS', category: 'Инфраструктура', amount: '49,90 $', currency: 'USD', frequency: 'ежемесячно', day: '25-е число', who: people.dima, nextDue: '25.10.2026' },
  { id: 'demo-t5', title: 'Хостинг бэкапов', category: 'Инфраструктура', amount: '12,00 $', currency: 'USD', frequency: 'ежемесячно', day: '20-е число', who: people.dima, nextDue: '20.11.2026' },
  { id: 'demo-t6', title: 'Дизайн-сервис', category: 'Подписки', amount: '15,00 €', currency: 'EUR', frequency: 'ежемесячно', day: '26-е число', who: people.anna, nextDue: '26.10.2026' },
  { id: 'demo-t7', title: 'Почта Workspace', category: 'Подписки', amount: '36,00 €', currency: 'EUR', frequency: 'ежемесячно', day: '30-е число', who: people.anna, nextDue: '30.10.2026' },
  { id: 'demo-t8', title: 'Старый CRM-сервис', category: 'Подписки', amount: '29,00 €', currency: 'EUR', frequency: 'ежемесячно', day: '2-е число', who: people.anna, ended: true },
]
