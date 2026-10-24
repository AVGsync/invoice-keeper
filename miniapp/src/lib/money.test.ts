import { describe, it, expect } from 'vitest'
import { formatMoney, parseMoneyInput, sumByCurrency, nonZeroCurrencies } from './money'

const NNBSP = ' '
const NBSP = ' '
const MINUS = '−'

describe('formatMoney', () => {
  it('группирует тысячи узким неразрывным пробелом и опускает ,00', () => {
    expect(formatMoney(120000, 'RUB')).toBe('1' + NNBSP + '200' + NBSP + '₽')
  })
  it('показывает дробь когда она не ноль', () => {
    expect(formatMoney(123456, 'RUB')).toBe('1' + NNBSP + '234,56' + NBSP + '₽')
  })
  it('значения меньше 100', () => {
    expect(formatMoney(5, 'USD')).toBe('0,05' + NBSP + '$')
    expect(formatMoney(99, 'EUR')).toBe('0,99' + NBSP + '€')
  })
  it('ноль', () => {
    expect(formatMoney(0, 'RUB')).toBe('0' + NBSP + '₽')
  })
  it('все три валюты', () => {
    expect(formatMoney(100000, 'RUB')).toBe('1' + NNBSP + '000' + NBSP + '₽')
    expect(formatMoney(100000, 'USD')).toBe('1' + NNBSP + '000' + NBSP + '$')
    expect(formatMoney(100000, 'EUR')).toBe('1' + NNBSP + '000' + NBSP + '€')
  })
  it('миллионы', () => {
    expect(formatMoney(123456789, 'USD')).toBe('1' + NNBSP + '234' + NNBSP + '567,89' + NBSP + '$')
  })
  it('отрицательное с настоящим минусом', () => {
    expect(formatMoney(-5000, 'RUB')).toBe(MINUS + '50' + NBSP + '₽')
  })
})

describe('parseMoneyInput', () => {
  it('1 234,5 → 123450', () => {
    expect(parseMoneyInput('1' + NNBSP + '234,5')).toBe(123450)
    expect(parseMoneyInput('1 234,5')).toBe(123450)
  })
  it('точка как разделитель', () => {
    expect(parseMoneyInput('10.99')).toBe(1099)
  })
  it('целое', () => {
    expect(parseMoneyInput('100')).toBe(10000)
  })
  it('null для нуля, отрицательного, >2 знаков, букв', () => {
    expect(parseMoneyInput('0')).toBeNull()
    expect(parseMoneyInput('-1')).toBeNull()
    expect(parseMoneyInput('1,234')).toBeNull()
    expect(parseMoneyInput('abc')).toBeNull()
    expect(parseMoneyInput('')).toBeNull()
  })
  it('верхняя граница', () => {
    expect(parseMoneyInput('100000000000')).toBe(1e13)
    expect(parseMoneyInput('100000000000.01')).toBeNull()
  })
})

describe('sumByCurrency', () => {
  it('никогда не складывает валюты вместе', () => {
    const items = [
      { currency: 'RUB' as const, amount: 100 },
      { currency: 'USD' as const, amount: 200 },
      { currency: 'RUB' as const, amount: 50 },
      { currency: 'EUR' as const, amount: 10 },
    ]
    const totals = sumByCurrency(items, (i) => i)
    expect(totals).toEqual({ RUB: 150, USD: 200, EUR: 10 })
  })
  it('nonZeroCurrencies в порядке RUB,USD,EUR', () => {
    expect(nonZeroCurrencies({ RUB: 0, USD: 5, EUR: 3 })).toEqual(['USD', 'EUR'])
    expect(nonZeroCurrencies({ RUB: 1, USD: 0, EUR: 0 })).toEqual(['RUB'])
  })
})
