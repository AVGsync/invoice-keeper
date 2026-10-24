import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  request,
  setToken,
  getToken,
  registerReauth,
  _resetSkewCapture,
  getSkew,
} from './client'
import { ApiError } from './errors'

function jsonRes(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  })
}
function textRes(status: number, text: string, headers: Record<string, string> = {}): Response {
  return new Response(text, { status, headers: { 'Content-Type': 'text/plain', ...headers } })
}

describe('request', () => {
  beforeEach(() => {
    setToken('TOK')
    _resetSkewCapture()
    registerReauth(async () => {})
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('добавляет Bearer-заголовок', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonRes(200, { ok: 1 }))
    vi.stubGlobal('fetch', fetchMock)
    await request('GET', '/auth/me')
    const init = fetchMock.mock.calls[0]![1]
    expect(init.headers['Authorization']).toBe('Bearer TOK')
    expect(init.headers['Accept']).toBe('application/json')
  })

  it('JSON-конверт → code/requestId', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonRes(409, { error: { code: 'telegram_not_linked', message: 'нет', request_id: 'r1' } }),
      ),
    )
    const err = (await request('POST', '/telegram/miniapp/auth', {
      body: {},
      auth: false,
    }).catch((e) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(409)
    expect(err.code).toBe('telegram_not_linked')
    expect(err.requestId).toBe('r1')
  })

  it('plain text → message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(textRes(403, 'forbidden')))
    const err = (await request('GET', '/users').catch((e) => e)) as ApiError
    expect(err.status).toBe(403)
    expect(err.message).toBe('forbidden')
    expect(err.code).toBeUndefined()
  })

  it('204 → undefined', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    const r = await request('PATCH', '/payment-occurrences/x', { body: { action: 'confirm' } })
    expect(r).toBeUndefined()
  })

  it('Retry-After', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(textRes(429, 'too many', { 'Retry-After': '42' })))
    const err = (await request('GET', '/users').catch((e) => e)) as ApiError
    expect(err.status).toBe(429)
    expect(err.retryAfter).toBe(42)
  })

  it('строит query, пропуская undefined', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonRes(200, {}))
    vi.stubGlobal('fetch', fetchMock)
    await request('GET', '/x', { query: { a: 'b', c: undefined, d: 1, e: true } })
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/x?a=b&d=1&e=true')
  })

  it('Date-заголовок задаёт skew', async () => {
    const future = new Date(Date.now() + 100000).toUTCString()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(200, {}, { Date: future })))
    await request('GET', '/x')
    expect(Math.abs(getSkew())).toBeGreaterThan(1000)
  })

  it('401 → один reauth → повтор успешен', async () => {
    let call = 0
    const fetchMock = vi.fn().mockImplementation(() => {
      call++
      if (call === 1) return Promise.resolve(textRes(401, 'unauthorized'))
      return Promise.resolve(jsonRes(200, { ok: 1 }))
    })
    vi.stubGlobal('fetch', fetchMock)
    const reauth = vi.fn().mockImplementation(async () => {
      setToken('NEW')
    })
    registerReauth(reauth)
    const r = await request<{ ok: number }>('GET', '/auth/me')
    expect(r.ok).toBe(1)
    expect(reauth).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('401 дважды → без цикла, выбрасывает', async () => {
    const fetchMock = vi.fn().mockResolvedValue(textRes(401, 'unauthorized'))
    vi.stubGlobal('fetch', fetchMock)
    const reauth = vi.fn().mockResolvedValue(undefined)
    registerReauth(reauth)
    const err = (await request('GET', '/auth/me').catch((e) => e)) as ApiError
    expect(err.status).toBe(401)
    expect(reauth).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('3 параллельных 401 → ровно один вызов /telegram/miniapp/auth', async () => {
    const authCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/telegram/miniapp/auth')) {
        authCalls.push(url)
        return Promise.resolve(jsonRes(200, { access_token: 'NEW' }))
      }
      // защищённые маршруты: 401 пока токен старый
      if (getSkewToken() === 'NEW') return Promise.resolve(jsonRes(200, { ok: 1 }))
      return Promise.resolve(textRes(401, 'unauthorized'))
    })
    vi.stubGlobal('fetch', fetchMock)

    // single-flight reauth как в session
    let inflight: Promise<void> | null = null
    registerReauth(() => {
      if (inflight) return inflight
      inflight = (async () => {
        await fetch('/api/v1/telegram/miniapp/auth', { method: 'POST' })
        setToken('NEW')
      })().finally(() => {
        inflight = null
      })
      return inflight
    })

    setToken('OLD')
    await Promise.all([
      request('GET', '/payment-occurrences'),
      request('GET', '/auth/me'),
      request('GET', '/users'),
    ])
    expect(authCalls.length).toBe(1)
  })

  it('никакой console.* не содержит initData', async () => {
    const spy = vi.spyOn(console, 'log').mockImplementation(() => {})
    const spyE = vi.spyOn(console, 'error').mockImplementation(() => {})
    const INIT = 'user=secret&hash=abc'
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(200, {})))
    await request('POST', '/telegram/miniapp/auth', { body: { init_data: INIT }, auth: false })
    for (const call of [...spy.mock.calls, ...spyE.mock.calls]) {
      expect(JSON.stringify(call)).not.toContain(INIT)
    }
  })
})

function getSkewToken(): string | null {
  return getToken()
}
