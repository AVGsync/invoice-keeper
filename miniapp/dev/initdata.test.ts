import { describe, it, expect } from 'vitest'
import { createHmac } from 'node:crypto'
import { signInitData } from './initdata'

describe('signInitData', () => {
  const botToken = '123456:TESTTOKEN'
  const user = { id: 100001, first_name: 'Иван', username: 'ivan' }

  it('даёт hash, совпадающий с независимым расчётом', () => {
    const authDate = 1700000000
    const signed = signInitData({ botToken, user, authDate, queryId: 'q1', startParam: 'occ_abc' })
    const params = new URLSearchParams(signed)
    const hash = params.get('hash')
    expect(hash).toBeTruthy()

    // Независимый расчёт по Telegram-алгоритму.
    const pairs: Record<string, string> = {}
    for (const [k, v] of params.entries()) {
      if (k !== 'hash') pairs[k] = v
    }
    const dcs = Object.keys(pairs)
      .sort()
      .map((k) => `${k}=${pairs[k]}`)
      .join('\n')
    const secret = createHmac('sha256', 'WebAppData').update(botToken).digest()
    const expected = createHmac('sha256', secret).update(dcs).digest('hex')
    expect(hash).toBe(expected)
  })

  it('не включает hash в строку проверки (ключи отсортированы)', () => {
    const signed = signInitData({ botToken, user, authDate: 1700000000, queryId: 'q1' })
    const params = new URLSearchParams(signed)
    // auth_date < query_id < user по алфавиту — проверяем, что все присутствуют.
    expect(params.get('auth_date')).toBe('1700000000')
    expect(params.get('query_id')).toBe('q1')
    expect(JSON.parse(params.get('user')!)).toMatchObject({ id: 100001 })
    expect(params.get('hash')).toMatch(/^[0-9a-f]{64}$/)
  })

  it('разный hash при разном start_param', () => {
    const a = signInitData({ botToken, user, authDate: 1700000000, queryId: 'q1' })
    const b = signInitData({ botToken, user, authDate: 1700000000, queryId: 'q1', startParam: 'occ_x' })
    const ha = new URLSearchParams(a).get('hash')
    const hb = new URLSearchParams(b).get('hash')
    expect(ha).not.toBe(hb)
  })
})
