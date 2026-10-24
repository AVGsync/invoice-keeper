import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  today,
  setSkew,
  addDays,
  diffDays,
  weekStart,
  weekday,
  monthGrid,
  monthRange,
  formatDate,
  formatShort,
  relativeDay,
  weekLabel,
  formatMonth,
  addMonths,
} from './dates'

describe('today', () => {
  beforeEach(() => setSkew(0))
  it('учитывает полночь по Москве', () => {
    // 21:30Z = 00:30 МСК следующего дня
    expect(today(Date.parse('2026-10-23T21:30:00Z'))).toBe('2026-10-24')
    // 20:59Z = 23:59 МСК того же дня
    expect(today(Date.parse('2026-10-23T20:59:00Z'))).toBe('2026-10-23')
  })
  it('учитывает skew (через аргумент по умолчанию)', () => {
    const base = Date.parse('2026-10-23T12:00:00Z')
    const spy = vi.spyOn(Date, 'now').mockReturnValue(base)
    setSkew(24 * 3600 * 1000)
    expect(today()).toBe('2026-10-24')
    setSkew(0)
    expect(today()).toBe('2026-10-23')
    spy.mockRestore()
  })
})

describe('addDays', () => {
  it('через месяц и год', () => {
    expect(addDays('2026-10-31', 1)).toBe('2026-11-01')
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2027-01-01', -1)).toBe('2026-12-31')
  })
})

describe('diffDays', () => {
  it('разница в днях', () => {
    expect(diffDays('2026-10-24', '2026-10-20')).toBe(4)
    expect(diffDays('2026-10-20', '2026-10-24')).toBe(-4)
  })
})

describe('weekday / weekStart', () => {
  it('0 = понедельник', () => {
    // 2026-10-24 — суббота
    expect(weekday('2026-10-24')).toBe(5)
    expect(weekStart('2026-10-24')).toBe('2026-10-19')
    // понедельник
    expect(weekday('2026-10-19')).toBe(0)
    expect(weekStart('2026-10-19')).toBe('2026-10-19')
  })
})

describe('monthGrid', () => {
  it('42 ячейки, первая 2026-09-28', () => {
    const grid = monthGrid('2026-10')
    expect(grid.length).toBe(42)
    expect(grid[0]).toBe('2026-09-28')
  })
  it('monthRange', () => {
    expect(monthRange('2026-10')).toEqual({ from: '2026-10-01', to: '2026-10-31' })
    expect(monthRange('2026-02')).toEqual({ from: '2026-02-01', to: '2026-02-28' })
  })
  it('addMonths', () => {
    expect(addMonths('2026-10', 1)).toBe('2026-11')
    expect(addMonths('2026-12', 1)).toBe('2027-01')
    expect(addMonths('2026-01', -1)).toBe('2025-12')
  })
})

describe('formatDate', () => {
  it('без года в текущем году', () => {
    expect(formatDate('2026-10-24', { today: '2026-10-24' })).toBe('24 октября')
  })
  it('с годом если отличается', () => {
    expect(formatDate('2025-10-24', { today: '2026-10-24' })).toBe('24 октября 2025')
  })
  it('formatShort', () => {
    expect(formatShort('2026-10-24')).toBe('24 окт.')
  })
})

describe('relativeDay', () => {
  it('сегодня/завтра/вчера', () => {
    expect(relativeDay('2026-10-24', '2026-10-24')).toBe('сегодня')
    expect(relativeDay('2026-10-25', '2026-10-24')).toBe('завтра')
    expect(relativeDay('2026-10-23', '2026-10-24')).toBe('вчера')
  })
  it('через N дней', () => {
    expect(relativeDay('2026-10-27', '2026-10-24')).toBe('через 3 дня')
    expect(relativeDay('2026-10-29', '2026-10-24')).toBe('через 5 дней')
  })
  it('просрочено', () => {
    expect(relativeDay('2026-10-19', '2026-10-24')).toBe('просрочено на 5 дн.')
  })
})

describe('weekLabel', () => {
  it('относительные недели', () => {
    expect(weekLabel('2026-10-19', '2026-10-24')).toBe('Эта неделя')
    expect(weekLabel('2026-10-26', '2026-10-24')).toBe('Следующая неделя')
    expect(weekLabel('2026-10-12', '2026-10-24')).toBe('Прошлая неделя')
  })
  it('диапазон дат', () => {
    expect(weekLabel('2026-11-02', '2026-10-24')).toBe('2–8 нояб.')
  })
})

describe('formatMonth', () => {
  it('Октябрь 2026', () => {
    expect(formatMonth('2026-10')).toBe('Октябрь 2026')
  })
})
