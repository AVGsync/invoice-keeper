// answers for the chat in demo mode: simple word matching over the demo payments.
// the real assistant lives on the server, this is only so the chat can be shown without it
import type { Currency, Period } from './demo'
import { demoToday } from './demo'
import { daysBetween, fromRu, lastDayOf, monthNames } from './date'

export const demoExamples = ['Что просрочено?', 'Сколько платим в долларах в этом месяце?', 'Кто отвечает за аренду?', 'Что оплатить на этой неделе?']

const currencyWords: [Currency, RegExp, string][] = [
  ['USD', /доллар|usd|\$/i, 'долларах'],
  ['EUR', /евро|eur|€/i, 'евро'],
  ['RUB', /рубл|rub|₽/i, 'рублях'],
]

const line = (period: Period) => `• ${period.title}: ${period.amount}, срок ${period.due}, ${period.who.short}`

export function demoAnswer(question: string, periods: Period[]): { step: string, text: string } {
  const q = question.toLowerCase()

  if (/просроч|долг|не оплат/.test(q)) {
    const overdue = periods.filter(period => period.status === 'overdue')
    if (!overdue.length) return { step: 'Ищу просроченные платежи', text: 'Просроченных платежей нет, всё оплачено вовремя.' }
    return {
      step: 'Ищу просроченные платежи',
      text: `Просрочено: ${overdue.length}.\n${overdue.map(period => `${line(period)}, просрочка ${period.overdue}`).join('\n')}`,
    }
  }

  const currency = currencyWords.find(([, words]) => words.test(q))
  if (currency) {
    const [code, , word] = currency
    const month = demoToday.slice(0, 7)
    const list = periods.filter(period => period.currency === code && fromRu(period.due).startsWith(month))
    const total = totalsOn(lastDayOf(month), list).find(item => item.code === code)!
    const monthName = monthNames[Number(month.slice(5)) - 1]!.toLowerCase()
    if (!list.length) return { step: 'Считаю суммы', text: `В ${word} за ${monthName} платежей нет.` }
    return {
      step: 'Считаю суммы',
      text: `В ${word} за ${monthName}: ${total.total} (${total.count}). Оплачено ${total.paid}, осталось ${total.unpaid}.\n${list.map(line).join('\n')}`,
    }
  }

  if (/недел|ближайш|скоро|оплатить/.test(q)) {
    const soon = periods.filter((period) => {
      const left = daysBetween(demoToday, fromRu(period.due))
      return !period.paidAt && left >= 0 && left <= 7
    })
    if (!soon.length) return { step: 'Смотрю ближайшие сроки', text: 'В ближайшие 7 дней платить нечего.' }
    return { step: 'Смотрю ближайшие сроки', text: `В ближайшие 7 дней: ${soon.length}.\n${soon.map(line).join('\n')}` }
  }

  // "кто отвечает за аренду": find the payment by any word of its title (first 4 letters, so endings don't matter)
  const found = periods.find(period => period.title.toLowerCase().split(/\s+/)
    .some(word => word.length > 3 && q.includes(word.slice(0, 4))))
  if (found) {
    return {
      step: 'Ищу платёж',
      text: `${found.title}: отвечает ${found.who.name} (${found.who.persona}). ${found.amount}, ${found.frequency}, ближайший срок ${found.due}.`,
    }
  }

  return {
    step: 'Разбираю вопрос',
    text: 'Не понял вопрос. В демо-режиме я знаю про просрочки, суммы по валютам, ближайшие сроки и ответственных.',
  }
}
