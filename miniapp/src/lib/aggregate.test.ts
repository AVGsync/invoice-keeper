import { describe, it, expect } from 'vitest'
import { monthTotals, groupByWeek, upcoming, overdue } from './aggregate'
import type { PaymentResponse } from '../api/types'

function occ(p: Partial<PaymentResponse>): PaymentResponse {
  return {
    id: p.id || 'x',
    user_id: p.user_id || 'u1',
    amount: p.amount ?? 1000,
    currency: p.currency || 'RUB',
    description: p.description || 'test',
    due_date: p.due_date || '2026-10-15',
    status: p.status || 'pending',
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...p,
  }
}

describe('monthTotals', () => {
  it('правила сводки; ended без оплаты не считается', () => {
    const items = [
      occ({ amount: 100, currency: 'RUB', status: 'paid', due_date: '2026-10-05' }),
      occ({ amount: 200, currency: 'RUB', status: 'overdue', due_date: '2026-10-06' }),
      occ({ amount: 50, currency: 'RUB', status: 'pending', due_date: '2026-10-07' }),
      occ({ amount: 999, currency: 'RUB', status: 'ended', due_date: '2026-10-08' }),
      occ({ amount: 7, currency: 'USD', status: 'paid', due_date: '2026-10-09' }),
      occ({ amount: 1, currency: 'RUB', status: 'pending', due_date: '2026-11-01' }), // вне месяца
    ]
    const t = monthTotals(items, '2026-10')
    expect(t.RUB).toEqual({ paid: 100, outstanding: 250, overdue: 200 })
    expect(t.USD).toEqual({ paid: 7, outstanding: 0, overdue: 0 })
  })
})

describe('groupByWeek', () => {
  it('группирует по неделям с суммами по валютам', () => {
    const items = [
      occ({ due_date: '2026-10-20', amount: 100, currency: 'RUB' }),
      occ({ due_date: '2026-10-21', amount: 50, currency: 'USD' }),
      occ({ due_date: '2026-10-27', amount: 10, currency: 'RUB' }),
    ]
    const groups = groupByWeek(items, '2026-10-24')
    expect(groups.length).toBe(2)
    expect(groups[0]!.weekStart).toBe('2026-10-19')
    expect(groups[0]!.totals.RUB).toBe(100)
    expect(groups[0]!.totals.USD).toBe(50)
    expect(groups[1]!.weekStart).toBe('2026-10-26')
  })
})

describe('upcoming', () => {
  it('pending в окне 7 дней', () => {
    const items = [
      occ({ id: 'a', status: 'pending', due_date: '2026-10-24' }),
      occ({ id: 'b', status: 'pending', due_date: '2026-10-30' }),
      occ({ id: 'c', status: 'pending', due_date: '2026-10-31' }), // вне окна (today+7)
      occ({ id: 'd', status: 'overdue', due_date: '2026-10-24' }), // не pending
    ]
    const up = upcoming(items, '2026-10-24', 7)
    expect(up.map((i) => i.id)).toEqual(['a', 'b'])
  })
})

describe('overdue', () => {
  it('только overdue', () => {
    const items = [
      occ({ id: 'a', status: 'overdue', due_date: '2026-10-10' }),
      occ({ id: 'b', status: 'pending' }),
    ]
    expect(overdue(items).map((i) => i.id)).toEqual(['a'])
  })
})
