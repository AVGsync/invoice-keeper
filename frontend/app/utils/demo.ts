// fake data copied from the figma screens.
// backend has no payment endpoints yet, swap this for useApi() once it does
import { fromRu } from './date'
import { formatAmount, formatMoney, parseMoney, plural } from './money'

// on the demo screens "today" is 24.10.2026
export const demoToday = '2026-10-24'

export type PeriodStatus = 'pending' | 'paid' | 'late' | 'overdue'
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
  title: string
  frequency: string
  meta: string
  who: Person
  due: string
  relative?: string
  amount: string
  currency: Currency
  status: PeriodStatus
  paidAt?: string
  overdue?: string
  // date the telegram reminder went out
  reminded?: string
}

export const periods: Period[] = [
  { title: 'Интернет в офисе', frequency: 'ежемесячно', meta: 'ежемесячно · офис', who: people.olga, due: '05.10.2026', amount: '1 500,00 ₽', currency: 'RUB', status: 'paid', paidAt: '05.10.2026' },
  { title: 'Аренда офиса', frequency: 'ежемесячно', meta: 'ежемесячно · офис', who: people.olga, due: '18.10.2026', relative: '6 дн. назад', amount: '120 000,00 ₽', currency: 'RUB', status: 'overdue', overdue: '6 дн.' },
  { title: 'Хостинг бэкапов', frequency: 'ежемесячно', meta: 'ежемесячно · хостинг', who: people.dima, due: '20.10.2026', amount: '12,00 $', currency: 'USD', status: 'late', paidAt: '22.10.2026', overdue: '+2 дн.' },
  { title: 'Сервер VPS', frequency: 'ежемесячно', meta: 'ежемесячно · хостинг', who: people.dima, due: '25.10.2026', relative: 'завтра', amount: '49,90 $', currency: 'USD', status: 'pending', reminded: '22.10' },
  { title: 'Дизайн-сервис', frequency: 'ежемесячно', meta: 'ежемесячно · подписка', who: people.anna, due: '26.10.2026', relative: 'через 2 дн.', amount: '15,00 €', currency: 'EUR', status: 'paid', paidAt: '20.10.2026' },
  { title: 'Почта Workspace', frequency: 'ежемесячно', meta: 'ежемесячно · подписка', who: people.anna, due: '30.10.2026', relative: 'через 6 дн.', amount: '36,00 €', currency: 'EUR', status: 'pending' },
  { title: 'Домен invoice-keeper.ru', frequency: 'ежегодно', meta: 'ежегодно · инфраструктура', who: people.dima, due: '31.10.2026', relative: 'через 7 дн.', amount: '1 990,00 ₽', currency: 'RUB', status: 'pending' },
]

// payments due in the next 7 days (24.10 is "today" on the board)
export const upcoming = periods.filter(period => Number(period.due.slice(0, 2)) >= 25)

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

export const totals: CurrencyTotal[] = [
  { code: 'RUB', name: 'Российский рубль', count: '3 платежа', total: '123 490,00 ₽', paid: '1 500,00', unpaid: '121 990,00', overdue: '120 000,00', progress: 0.012 },
  { code: 'USD', name: 'Доллар США', count: '2 платежа', total: '61,90 $', paid: '12,00', unpaid: '49,90', progress: 0.19 },
  { code: 'EUR', name: 'Евро', count: '2 платежа', total: '51,00 €', paid: '15,00', unpaid: '36,00', progress: 0.29 },
]

const currencyNames: Record<Currency, string> = { RUB: 'Российский рубль', USD: 'Доллар США', EUR: 'Евро' }

// totals per currency on a date: what was due by then, plus what got paid early.
// the backend should answer this itself (e.g. GET /payments/summary?date=), here it's only for the demo
export function totalsOn(dateIso: string, list: Period[] = periods): CurrencyTotal[] {
  const paidBy = (period: Period) => !!period.paidAt && fromRu(period.paidAt) <= dateIso
  const sum = (list: Period[]) => list.reduce((total, period) => total + parseMoney(period.amount), 0)

  return (Object.keys(currencyNames) as Currency[]).map((code) => {
    const counted = list.filter(p => p.currency === code && (fromRu(p.due) <= dateIso || paidBy(p)))
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

export const templates: Template[] = [
  { title: 'Аренда офиса', category: 'Офис', amount: '120 000,00 ₽', currency: 'RUB', frequency: 'ежемесячно', day: '18-е число', who: people.olga, nextDue: '18.11.2026' },
  { title: 'Интернет в офисе', category: 'Офис', amount: '1 500,00 ₽', currency: 'RUB', frequency: 'ежемесячно', day: '5-е число', who: people.olga, nextDue: '05.11.2026' },
  { title: 'Домен invoice-keeper.ru', category: 'Инфраструктура', amount: '1 990,00 ₽', currency: 'RUB', frequency: 'ежегодно', day: '31 октября', who: people.dima, nextDue: '31.10.2026' },
  { title: 'Сервер VPS', category: 'Инфраструктура', amount: '49,90 $', currency: 'USD', frequency: 'ежемесячно', day: '25-е число', who: people.dima, nextDue: '25.10.2026' },
  { title: 'Хостинг бэкапов', category: 'Инфраструктура', amount: '12,00 $', currency: 'USD', frequency: 'ежемесячно', day: '20-е число', who: people.dima, nextDue: '20.11.2026' },
  { title: 'Дизайн-сервис', category: 'Подписки', amount: '15,00 €', currency: 'EUR', frequency: 'ежемесячно', day: '26-е число', who: people.anna, nextDue: '26.10.2026' },
  { title: 'Почта Workspace', category: 'Подписки', amount: '36,00 €', currency: 'EUR', frequency: 'ежемесячно', day: '30-е число', who: people.anna, nextDue: '30.10.2026' },
  { title: 'Старый CRM-сервис', category: 'Подписки', amount: '29,00 €', currency: 'EUR', frequency: 'ежемесячно', day: '2-е число', who: people.anna, ended: true },
]
