import { createHmac, randomBytes } from 'node:crypto'

// Подписывает dev-initData по алгоритму Telegram WebApp.

export interface DevUser {
  id: number
  first_name: string
  username?: string
  language_code?: string
  allows_write_to_pm?: boolean
}

export interface SignOptions {
  botToken: string
  user: DevUser
  startParam?: string
  authDate?: number // секунды Unix
  queryId?: string
}

export function signInitData(opts: SignOptions): string {
  const authDate = opts.authDate ?? Math.floor(Date.now() / 1000)
  const queryId = opts.queryId ?? randomBytes(8).toString('hex')

  const pairs: Record<string, string> = {
    auth_date: String(authDate),
    query_id: queryId,
    user: JSON.stringify(opts.user),
  }
  if (opts.startParam) pairs.start_param = opts.startParam

  // dataCheckString: ключи отсортированы, k=v через \n, hash в проверку не входит.
  const dataCheckString = Object.keys(pairs)
    .sort()
    .map((k) => `${k}=${pairs[k]}`)
    .join('\n')

  const secret = createHmac('sha256', 'WebAppData').update(opts.botToken).digest()
  const hash = createHmac('sha256', secret).update(dataCheckString).digest('hex')

  // Результат — URL-кодированные пары + hash.
  const params = new URLSearchParams()
  for (const k of Object.keys(pairs)) {
    const v = pairs[k]
    if (v !== undefined) params.set(k, v)
  }
  params.set('hash', hash)
  return params.toString()
}
