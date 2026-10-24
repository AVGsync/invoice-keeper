// Работа только со строками YYYY-MM-DD.

export const APP_TZ = import.meta.env.VITE_APP_TIMEZONE || 'Europe/Moscow'

let skewMs = 0

export function setSkew(ms: number): void {
  skewMs = ms
}

export function getSkew(): number {
  return skewMs
}

export function today(now: number = Date.now() + skewMs, tz: string = APP_TZ): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: tz,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(now)
}

function parse(d: string): { y: number; m: number; day: number } {
  const y = Number(d.slice(0, 4))
  const m = Number(d.slice(5, 7))
  const day = Number(d.slice(8, 10))
  return { y, m, day }
}

function toUTC(d: string): number {
  const { y, m, day } = parse(d)
  return Date.UTC(y, m - 1, day)
}

function fromUTC(ms: number): string {
  const dt = new Date(ms)
  const y = dt.getUTCFullYear()
  const m = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const day = String(dt.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

const DAY = 86400000

export function addDays(d: string, n: number): string {
  return fromUTC(toUTC(d) + n * DAY)
}

export function diffDays(a: string, b: string): number {
  return Math.round((toUTC(a) - toUTC(b)) / DAY)
}

// 0 = понедельник.
export function weekday(d: string): number {
  const js = new Date(toUTC(d)).getUTCDay() // 0 = воскресенье
  return (js + 6) % 7
}

export function weekStart(d: string): string {
  return addDays(d, -weekday(d))
}

export function monthKey(d: string): string {
  return d.slice(0, 7)
}

export function monthRange(ym: string): { from: string; to: string } {
  const y = Number(ym.slice(0, 4))
  const m = Number(ym.slice(5, 7))
  const from = `${ym}-01`
  const lastDay = new Date(Date.UTC(y, m, 0)).getUTCDate()
  const to = `${ym}-${String(lastDay).padStart(2, '0')}`
  return { from, to }
}

// 42 ячейки (6 недель), начиная с понедельника.
export function monthGrid(ym: string): string[] {
  const { from } = monthRange(ym)
  const start = weekStart(from)
  const cells: string[] = []
  for (let i = 0; i < 42; i++) cells.push(addDays(start, i))
  return cells
}

export function addMonths(ym: string, n: number): string {
  const y = Number(ym.slice(0, 4))
  const m = Number(ym.slice(5, 7))
  const total = y * 12 + (m - 1) + n
  const ny = Math.floor(total / 12)
  const nm = (total % 12) + 1
  return `${ny}-${String(nm).padStart(2, '0')}`
}

const MONTHS_GEN = [
  'января',
  'февраля',
  'марта',
  'апреля',
  'мая',
  'июня',
  'июля',
  'августа',
  'сентября',
  'октября',
  'ноября',
  'декабря',
]

const MONTHS_SHORT = [
  'янв.',
  'февр.',
  'марта',
  'апр.',
  'мая',
  'июня',
  'июля',
  'авг.',
  'сент.',
  'окт.',
  'нояб.',
  'дек.',
]

const MONTHS_NOM = [
  'Январь',
  'Февраль',
  'Март',
  'Апрель',
  'Май',
  'Июнь',
  'Июль',
  'Август',
  'Сентябрь',
  'Октябрь',
  'Ноябрь',
  'Декабрь',
]

export function formatDate(d: string, opts: { today: string }): string {
  const { m, day } = parse(d)
  const base = `${day} ${MONTHS_GEN[m - 1]}`
  const curYear = Number(opts.today.slice(0, 4))
  const dYear = Number(d.slice(0, 4))
  return dYear === curYear ? base : `${base} ${dYear}`
}

export function formatShort(d: string): string {
  const { m, day } = parse(d)
  return `${day} ${MONTHS_SHORT[m - 1]}`
}

function plural(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few
  return many
}

export function relativeDay(d: string, todayStr: string): string {
  const delta = diffDays(d, todayStr)
  if (delta === 0) return 'сегодня'
  if (delta === 1) return 'завтра'
  if (delta === -1) return 'вчера'
  if (delta > 1) {
    const w = plural(delta, 'день', 'дня', 'дней')
    return `через ${delta} ${w}`
  }
  const n = -delta
  const w = plural(n, 'дн.', 'дн.', 'дн.')
  return `просрочено на ${n} ${w}`
}

export function weekLabel(start: string, todayStr: string): string {
  const curWeek = weekStart(todayStr)
  const delta = diffDays(start, curWeek)
  if (delta === 0) return 'Эта неделя'
  if (delta === 7) return 'Следующая неделя'
  if (delta === -7) return 'Прошлая неделя'
  const end = addDays(start, 6)
  const s = parse(start)
  const e = parse(end)
  if (s.m === e.m) {
    return `${s.day}–${e.day} ${MONTHS_SHORT[e.m - 1]}`
  }
  return `${s.day} ${MONTHS_SHORT[s.m - 1]} – ${e.day} ${MONTHS_SHORT[e.m - 1]}`
}

export function formatMonth(ym: string): string {
  const y = Number(ym.slice(0, 4))
  const m = Number(ym.slice(5, 7))
  return `${MONTHS_NOM[m - 1]} ${y}`
}

export function formatDateTime(rfc3339: string): string {
  const dt = new Date(rfc3339)
  if (Number.isNaN(dt.getTime())) return rfc3339
  return new Intl.DateTimeFormat('ru-RU', {
    timeZone: APP_TZ,
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(dt)
}
