import { describe, it, expect } from 'vitest'
import { parseStartParam } from './startParam'

describe('parseStartParam', () => {
  it('валидный hex32 → uuid 8-4-4-4-12', () => {
    expect(parseStartParam('occ_0123456789abcdef0123456789abcdef')).toEqual({
      occId: '01234567-89ab-cdef-0123-456789abcdef',
    })
  })
  it('верхний регистр приводится к нижнему', () => {
    expect(parseStartParam('occ_0123456789ABCDEF0123456789ABCDEF')).toEqual({
      occId: '01234567-89ab-cdef-0123-456789abcdef',
    })
  })
  it('неверная длина → null', () => {
    expect(parseStartParam('occ_0123')).toBeNull()
    expect(parseStartParam('occ_0123456789abcdef0123456789abcde')).toBeNull()
  })
  it('не-hex → null', () => {
    expect(parseStartParam('occ_zzzz456789abcdef0123456789abcdef')).toBeNull()
  })
  it('неверный префикс → null', () => {
    expect(parseStartParam('pay_0123456789abcdef0123456789abcdef')).toBeNull()
    expect(parseStartParam('0123456789abcdef0123456789abcdef')).toBeNull()
  })
  it('пусто → null', () => {
    expect(parseStartParam(null)).toBeNull()
    expect(parseStartParam('')).toBeNull()
    expect(parseStartParam(undefined)).toBeNull()
  })
})
