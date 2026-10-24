import { describe, it, expect } from 'vitest'
import { canMarkPaid, canAdmin } from './permissions'

describe('canMarkPaid', () => {
  it('админ может всё', () => {
    expect(canMarkPaid({ id: 'a', role: 'admin' }, { user_id: 'other' })).toBe(true)
  })
  it('пользователь — только свои', () => {
    expect(canMarkPaid({ id: 'u1', role: 'user' }, { user_id: 'u1' })).toBe(true)
    expect(canMarkPaid({ id: 'u1', role: 'user' }, { user_id: 'u2' })).toBe(false)
  })
  it('нет пользователя — нельзя', () => {
    expect(canMarkPaid(null, { user_id: 'u1' })).toBe(false)
  })
})

describe('canAdmin', () => {
  it('роль', () => {
    expect(canAdmin({ role: 'admin' })).toBe(true)
    expect(canAdmin({ role: 'user' })).toBe(false)
    expect(canAdmin(null)).toBe(false)
  })
})
