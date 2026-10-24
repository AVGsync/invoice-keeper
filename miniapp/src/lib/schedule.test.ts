import { describe, it, expect } from 'vitest'
import { describeSchedule } from './schedule'

describe('describeSchedule', () => {
  it('monthly 1 — число из start_date', () => {
    expect(describeSchedule('monthly', 1, '2026-10-15')).toBe('Ежемесячно, 15-го числа')
  })
  it('monthly n', () => {
    expect(describeSchedule('monthly', 3, '2026-10-15')).toBe('Каждые 3 мес., 15-го числа')
  })
  it('weekly 1 — день недели из start_date (вторник)', () => {
    // 2026-10-20 — вторник
    expect(describeSchedule('weekly', 1, '2026-10-20')).toBe('Еженедельно, по вторникам')
  })
  it('weekly n', () => {
    expect(describeSchedule('weekly', 2, '2026-10-20')).toBe('Каждые 2 нед., по вторникам')
  })
})
