import { describe, it, expect } from 'vitest'
import { humanError } from './humanError'
import { ApiError } from '../api/errors'

describe('humanError', () => {
  it('network', () => {
    expect(humanError(new ApiError(0, 'fail', { network: true }))).toBe('Нет соединения с сервером')
  })
  it('403', () => {
    expect(humanError(new ApiError(403, 'forbidden'))).toBe('Недостаточно прав для этого действия')
  })
  it('404', () => {
    expect(humanError(new ApiError(404, 'not found'))).toBe('Не найдено')
  })
  it('429 с Retry-After', () => {
    expect(humanError(new ApiError(429, 'rate', { retryAfter: 30 }))).toBe(
      'Слишком много запросов, подождите 30 с',
    )
  })
  it('500', () => {
    expect(humanError(new ApiError(503, 'x'))).toBe('Сервер недоступен, попробуйте позже')
  })
  it('400 будущая дата', () => {
    expect(humanError(new ApiError(400, 'invalid payment: paid date is in the future'))).toBe(
      'Дата оплаты не может быть в будущем',
    )
  })
  it('409 уже оплачено', () => {
    expect(humanError(new ApiError(409, 'payment already paid'))).toBe(
      'Платёж уже отмечен оплаченным',
    )
  })
  it('409 логин занят', () => {
    expect(humanError(new ApiError(409, 'user with this login already exists'))).toBe(
      'Этот логин уже занят',
    )
  })
  it('400 amount', () => {
    expect(humanError(new ApiError(400, 'amount must be > 0'))).toBe('Сумма должна быть больше нуля')
  })
  it('400 login', () => {
    expect(humanError(new ApiError(400, 'login must be 3-32'))).toBe(
      'Логин: 3–32 символа, латиница, цифры, . _ -',
    )
  })
  it('400 прочее', () => {
    expect(humanError(new ApiError(400, 'weird thing'))).toBe('Ошибка в данных: weird thing')
  })
  it('не ApiError', () => {
    expect(humanError(new Error('x'))).toBe('Что-то пошло не так')
  })
})
