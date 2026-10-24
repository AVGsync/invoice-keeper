import { describe, it, expect } from 'vitest'
import { statusInfo, fromAssistant } from './status'

describe('statusInfo', () => {
  it('метки и цвета', () => {
    expect(statusInfo({ status: 'pending' }).label).toBe('Ожидает')
    expect(statusInfo({ status: 'overdue' }).label).toBe('Просрочен')
    expect(statusInfo({ status: 'paid', paid_late: false }).label).toBe('Оплачен')
    expect(statusInfo({ status: 'paid', paid_late: true }).label).toBe('Оплачен с опозданием')
    expect(statusInfo({ status: 'ended' }).label).toBe('Завершён')
    expect(statusInfo({ status: 'pending' }).colorVar).toBe('--c-pending')
  })
})

describe('fromAssistant', () => {
  it('upcoming → pending', () => {
    expect(fromAssistant('upcoming')).toBe('pending')
    expect(fromAssistant('overdue')).toBe('overdue')
    expect(fromAssistant('paid')).toBe('paid')
  })
})
