// dates move around as 'YYYY-MM-DD' (no time, so no timezone drift), screens show 'DD.MM.YYYY'

export const monthNames = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']

// for «Итоги октября», «31 октября»
export const monthNamesGenitive = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']

// months move around as 'YYYY-MM'
export function monthKey(year: number, month: number): string {
  const date = new Date(year, month, 1)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

export function parseMonthKey(key: string) {
  const [year, month] = key.split('-').map(Number) as [number, number]
  return { year, month: month - 1 }
}

// '2026-10' -> '2026-10-31'
export function lastDayOf(key: string): string {
  const { year, month } = parseMonthKey(key)
  return isoOf(year, month, new Date(year, month + 1, 0).getDate())
}

export function isoOf(year: number, month: number, day: number): string {
  return `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

// '2026-10-24' -> '24.10.2026'
export function toRu(iso?: string): string {
  if (!iso) return ''
  const [y, m, d] = iso.split('-')
  return `${d}.${m}.${y}`
}

// '24.10.2026' -> '2026-10-24', or '' if it isn't a real date (31.02 etc)
export function fromRu(text: string): string {
  const match = text.trim().match(/^(\d{2})\.(\d{2})\.(\d{4})$/)
  if (!match) return ''
  const [, d, m, y] = match.map(Number) as [number, number, number, number]
  const date = new Date(y, m - 1, d)
  if (date.getMonth() !== m - 1 || date.getDate() !== d) return ''
  return isoOf(y, m - 1, d)
}

// today in the app timezone
export function todayIso(): string {
  // en-CA formats as YYYY-MM-DD
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Moscow' }).format(new Date())
}

export function daysBetween(fromIso: string, toIso: string): number {
  return Math.round((Date.parse(toIso) - Date.parse(fromIso)) / 86_400_000)
}

// weeks of a month starting on monday, with the neighbour months' days around it
export function monthCells(year: number, month: number, weeks = 6) {
  const shift = (new Date(year, month, 1).getDay() + 6) % 7
  return Array.from({ length: weeks * 7 }, (_, i) => {
    const date = new Date(year, month, 1 - shift + i)
    return {
      iso: isoOf(date.getFullYear(), date.getMonth(), date.getDate()),
      day: date.getDate(),
      inMonth: date.getMonth() === month,
      weekend: i % 7 >= 5,
    }
  })
}

// next due dates for a preview. a day the month doesn't have becomes its last day,
// and the next month goes back to the original day: 31.01 -> 28.02 -> 31.03
export function nextDueDates(firstIso: string, count: number, yearly = false): string[] {
  const [y, m, d] = firstIso.split('-').map(Number) as [number, number, number]
  return Array.from({ length: count }, (_, i) => {
    const year = yearly ? y + i : y + Math.floor((m - 1 + i) / 12)
    const month = yearly ? m - 1 : (m - 1 + i) % 12
    const lastDay = new Date(year, month + 1, 0).getDate()
    return isoOf(year, month, Math.min(d, lastDay))
  })
}
