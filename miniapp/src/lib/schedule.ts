import type { Frequency } from '../api/types'

// Метка графика выводится из start_date, а не из day_offset.

const WEEKDAYS_BY = [
  'понедельникам',
  'вторникам',
  'средам',
  'четвергам',
  'пятницам',
  'субботам',
  'воскресеньям',
]

function weekdayMon0(d: string): number {
  const y = Number(d.slice(0, 4))
  const m = Number(d.slice(5, 7))
  const day = Number(d.slice(8, 10))
  const js = new Date(Date.UTC(y, m - 1, day)).getUTCDay()
  return (js + 6) % 7
}

export function describeSchedule(freq: Frequency, interval: number, startDate: string): string {
  if (freq === 'monthly') {
    const dayOfMonth = Number(startDate.slice(8, 10))
    if (interval === 1) return `Ежемесячно, ${dayOfMonth}-го числа`
    return `Каждые ${interval} мес., ${dayOfMonth}-го числа`
  }
  // weekly
  const wd = WEEKDAYS_BY[weekdayMon0(startDate)]
  if (interval === 1) return `Еженедельно, по ${wd}`
  return `Каждые ${interval} нед., по ${wd}`
}
