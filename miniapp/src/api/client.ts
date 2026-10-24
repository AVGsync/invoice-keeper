import { ApiError, parseError } from './errors'
import { setSkew, getSkew } from '../lib/dates'

// Токен — модульный let, не реактивный, живёт только в памяти JS (не в хранилищах браузера).
let token: string | null = null
let skewSet = false

export function setToken(t: string | null): void {
  token = t
}

export function getToken(): string | null {
  return token
}

// Регистрация reauth из session, чтобы избежать циклической зависимости.
type ReauthFn = () => Promise<void>
let reauthFn: ReauthFn | null = null
export function registerReauth(fn: ReauthFn): void {
  reauthFn = fn
}

export interface RequestOpts {
  body?: unknown
  query?: Record<string, string | number | boolean | undefined>
  signal?: AbortSignal
  auth?: boolean // по умолчанию true
  retried?: boolean
  raw?: boolean
  headers?: Record<string, string>
}

function buildUrl(path: string, query?: RequestOpts['query']): string {
  let url = '/api/v1' + path
  if (query) {
    const sp = new URLSearchParams()
    for (const k of Object.keys(query)) {
      const v = query[k]
      if (v !== undefined) sp.set(k, String(v))
    }
    const qs = sp.toString()
    if (qs) url += '?' + qs
  }
  return url
}

function captureSkew(res: Response): void {
  if (skewSet) return
  const date = res.headers.get('Date')
  if (date) {
    const parsed = Date.parse(date)
    if (!Number.isNaN(parsed)) {
      setSkew(parsed - Date.now())
      skewSet = true
    }
  }
}

export async function request<T>(
  method: string,
  path: string,
  opts: RequestOpts = {},
): Promise<T> {
  const auth = opts.auth !== false
  const url = buildUrl(path, opts.query)

  const headers: Record<string, string> = {
    Accept: 'application/json',
    'X-Client': 'miniapp',
    ...opts.headers,
  }
  let body: BodyInit | undefined
  if (opts.body instanceof FormData) {
    body = opts.body
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }
  if (auth && token) {
    headers['Authorization'] = 'Bearer ' + token
  }

  let res: Response
  try {
    res = await fetch(url, { method, headers, body, signal: opts.signal })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    if (e instanceof Error && e.name === 'AbortError') throw e
    throw new ApiError(0, 'network error', { network: true })
  }

  captureSkew(res)

  // 401 на auth-маршруте (не /telegram/miniapp/*) и без retried → один reauth.
  const isMiniAppRoute = path.startsWith('/telegram/miniapp/')
  if (res.status === 401 && auth && !isMiniAppRoute && !opts.retried && reauthFn) {
    await reauthFn()
    return request<T>(method, path, { ...opts, retried: true })
  }

  if (!res.ok) {
    throw await parseError(res)
  }

  if (opts.raw) {
    return res as unknown as T
  }

  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}


export { getSkew }
