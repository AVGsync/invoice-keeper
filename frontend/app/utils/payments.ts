// turns what the server sends (GET /payment-occurrences, GET /recurring-payments)
// into the same rows the screens already draw for the demo data
import type { PaymentOccurrence, RecurrentPayment } from '~/types/api'
import type { Currency, Period, PeriodStatus, Person, Template } from './demo'
import { daysBetween, isoOf, monthNamesGenitive, toRu } from './date'
import { formatMoney } from './money'
import { initials } from './format'

const weekdays = ['воскресеньям', 'понедельникам', 'вторникам', 'средам', 'четвергам', 'пятницам', 'субботам']

// "Ольга Иванова" -> "О. Иванова"
export function shortName(name: string): string {
  const words = name.trim().split(/\s+/)
  if (words.length < 2) return name
  return `${words[0]![0]}. ${words.slice(1).join(' ')}`
}

export function personOf(name = '', persona = ''): Person {
  return { initials: initials(name) || '—', short: shortName(name) || '—', name, persona }
}

// the server keeps kopecks / cents
export function money(minor = 0, currency = 'RUB'): string {
  return formatMoney(minor / 100, currency as Currency)
}

export function frequencyLabel(template?: RecurrentPayment): string {
  if (!template) return 'разовый'
  const every = template.interval || 1
  if (template.frequency === 'weekly') return every === 1 ? 'еженедельно' : `раз в ${every} нед.`
  if (every === 1) return 'ежемесячно'
  if (every === 3) return 'ежеквартально'
  if (every === 12) return 'ежегодно'
  return `раз в ${every} мес.`
}

// "18-е число", "31 октября" for yearly ones, "по средам" for weekly ones
export function dayLabel(template: RecurrentPayment): string {
  const start = template.start_date ?? ''
  if (template.frequency === 'weekly') return `по ${weekdays[template.day_offset ?? 0]}`
  const day = template.day_offset ?? Number(start.slice(8, 10))
  if (template.interval === 12 && start) return `${day} ${monthNamesGenitive[Number(start.slice(5, 7)) - 1]}`
  return `${day}-е число`
}

// same steps as the server: every N weeks, or every N months on the same day
// (the last day of the month when that month is shorter). gives `count` dates from fromIso on
export function dueDatesOf(template: RecurrentPayment, fromIso: string, count: number): string[] {
  const start = template.start_date
  if (!start) return []
  const every = template.interval || 1
  const [y, m, d] = start.split('-').map(Number) as [number, number, number]
  const out: string[] = []

  // a cap, so a broken template can't loop forever
  for (let k = 0; k < 1000 && out.length < count; k++) {
    let iso: string
    if (template.frequency === 'weekly') {
      const date = new Date(y, m - 1, d + 7 * every * k)
      iso = isoOf(date.getFullYear(), date.getMonth(), date.getDate())
    }
    else {
      const month = m - 1 + k * every
      const year = y + Math.floor(month / 12)
      const lastDay = new Date(year, (month % 12) + 1, 0).getDate()
      iso = isoOf(year, month % 12, Math.min(d, lastDay))
    }
    if (iso >= fromIso) out.push(iso)
  }
  return out
}

export function nextDueOf(template: RecurrentPayment, todayIso: string): string | undefined {
  return dueDatesOf(template, todayIso, 1)[0]
}

// "завтра", "через 3 дн.", "6 дн. назад"
export function relativeOf(dueIso: string, todayIso: string): string {
  const days = daysBetween(todayIso, dueIso)
  if (days === 0) return 'сегодня'
  if (days === 1) return 'завтра'
  if (days > 1) return `через ${days} дн.`
  return `${-days} дн. назад`
}

export function toPeriod(item: PaymentOccurrence, template: RecurrentPayment | undefined, persona: string, todayIso: string): Period {
  const due = item.due_date ?? ''
  const paid = item.paid_date ?? undefined
  const frequency = frequencyLabel(template)

  let status: PeriodStatus = 'pending'
  if (item.status === 'paid') status = item.paid_late ? 'late' : 'paid'
  else if (item.status === 'overdue') status = 'overdue'
  else if (item.status === 'ended') status = 'ended'

  let overdue: string | undefined
  if (status === 'overdue') overdue = `${daysBetween(due, todayIso)} дн.`
  if (status === 'late' && paid) overdue = `+${daysBetween(due, paid)} дн.`

  return {
    id: item.id ?? '',
    userId: item.user_id,
    templateId: item.recurrent_payment_id ?? undefined,
    title: item.description ?? '',
    frequency,
    meta: persona ? `${frequency} · ${persona.toLowerCase()}` : frequency,
    who: personOf(item.assignee_name, persona),
    due: toRu(due),
    relative: paid || status === 'ended' ? undefined : relativeOf(due, todayIso),
    amount: money(item.amount, item.currency),
    amountMinor: item.amount ?? 0,
    currency: (item.currency ?? 'RUB') as Currency,
    status,
    paidAt: paid ? toRu(paid) : undefined,
    overdue,
  }
}

export function toTemplate(item: RecurrentPayment, persona: string, todayIso: string): Template {
  const ended = !!item.stopped_at
  const next = ended ? undefined : nextDueOf(item, todayIso)
  return {
    id: item.id ?? '',
    assigneeId: item.assignee_id ?? undefined,
    title: item.description ?? '',
    category: persona,
    amount: money(item.amount, item.currency),
    currency: (item.currency ?? 'RUB') as Currency,
    frequency: frequencyLabel(item),
    day: dayLabel(item),
    who: personOf(item.assignee_name, persona),
    nextDue: next ? toRu(next) : undefined,
    ended,
  }
}
