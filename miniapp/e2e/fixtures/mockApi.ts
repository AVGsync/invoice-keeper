import type { Page, Route } from '@playwright/test'
import {
  USERS,
  USER_LIST,
  OCCURRENCES,
  RECURRING,
  PERSONAS,
  ASSISTANT_STATUS,
  type MockOcc,
  type MockUser,
} from './data'

export interface RecordedRequest {
  method: string
  path: string
  body: unknown
}

export interface MockApiOptions {
  actingUser?: keyof typeof USERS
  authMode?: 'ok' | 'needs_link' | 'expired'
  assistantEnabled?: boolean
  forbidPatch?: boolean
}

export interface MockApiHandle {
  requests: RecordedRequest[]
  find(method: string, pathIncludes: string): RecordedRequest | undefined
}

const DATE_HEADER = (): Record<string, string> => ({ Date: new Date('2026-10-24T06:00:00Z').toUTCString() })

function json(route: Route, status: number, body: unknown): Promise<void> {
  return route.fulfill({
    status,
    headers: { 'Content-Type': 'application/json', ...DATE_HEADER() },
    body: JSON.stringify(body),
  })
}
function text(route: Route, status: number, body: string): Promise<void> {
  return route.fulfill({
    status,
    headers: { 'Content-Type': 'text/plain', ...DATE_HEADER() },
    body,
  })
}
function noContent(route: Route): Promise<void> {
  return route.fulfill({ status: 204, headers: DATE_HEADER() })
}

function authResponse(user: MockUser): unknown {
  return {
    access_token: 'e2e-token-' + user.login,
    token_type: 'Bearer',
    expires_in: 3600,
    user: { id: user.id, full_name: user.full_name, role: user.role },
    dm_available: true,
  }
}

export async function installMockApi(page: Page, opts: MockApiOptions = {}): Promise<MockApiHandle> {
  const actingUser = USERS[opts.actingUser ?? 'user1']!
  let authMode = opts.authMode ?? 'ok'
  const assistantEnabled = opts.assistantEnabled ?? true
  const forbidPatch = opts.forbidPatch ?? false

  // Изолированные копии состояния на один тест.
  const occ: MockOcc[] = OCCURRENCES.map((o) => ({ ...o }))
  const recurring = RECURRING.map((r) => ({ ...r }))

  const handle: MockApiHandle = {
    requests: [],
    find(method, pathIncludes) {
      return handle.requests.find((r) => r.method === method && r.path.includes(pathIncludes))
    },
  }

  await page.route('**/api/v1/**', async (route) => {
    const req = route.request()
    const method = req.method()
    const url = new URL(req.url())
    const path = url.pathname.replace(/^\/api\/v1/, '')
    let body: unknown = undefined
    try {
      const raw = req.postData()
      if (raw) body = JSON.parse(raw)
    } catch {
      body = req.postData()
    }
    handle.requests.push({ method, path, body })

    // Auth
    if (method === 'POST' && path === '/telegram/miniapp/auth') {
      if (authMode === 'needs_link') return json(route, 409, { error: { code: 'telegram_not_linked', message: 'not linked' } })
      if (authMode === 'expired') return json(route, 401, { error: { code: 'init_data_expired', message: 'expired' } })
      return json(route, 200, authResponse(actingUser))
    }
    if (method === 'POST' && path === '/telegram/miniapp/link') {
      authMode = 'ok'
      return json(route, 200, authResponse(actingUser))
    }
    if (method === 'GET' && path === '/auth/me') {
      return json(route, 200, actingUser)
    }

    // Occurrences
    if (method === 'GET' && path === '/payment-occurrences') {
      return json(route, 200, { items: occ, total: occ.length })
    }
    const occIdMatch = path.match(/^\/payment-occurrences\/([^/]+)$/)
    if (occIdMatch) {
      const id = occIdMatch[1]!
      const item = occ.find((o) => o.id === id)
      if (method === 'GET') {
        if (!item) return text(route, 404, 'not found')
        return json(route, 200, item)
      }
      if (method === 'PATCH') {
        if (forbidPatch) return text(route, 403, 'forbidden')
        if (!item) return text(route, 404, 'not found')
        const action = (body as { action?: string; paid_date?: string }).action
        if (action === 'confirm') {
          const pd = (body as { paid_date?: string }).paid_date || '2026-10-24'
          item.paid_date = pd
          item.paid_late = pd > item.due_date
          item.status = 'paid'
          return noContent(route)
        }
        if (action === 'cancel') {
          delete item.paid_date
          item.paid_late = false
          item.status = item.due_date < '2026-10-24' ? 'overdue' : 'pending'
          return noContent(route)
        }
        return text(route, 400, 'action must be confirm or cancel')
      }
    }

    // Recurring
    if (method === 'GET' && path === '/recurring-payments') {
      return json(route, 200, recurring)
    }
    if (method === 'POST' && path === '/recurring-payments') {
      const id = '99999999-9999-4999-8999-999999999999'
      return json(route, 201, { id })
    }
    const recDel = path.match(/^\/recurring-payments\/([^/]+)$/)
    if (recDel && method === 'DELETE') {
      const r = recurring.find((x) => x.id === recDel[1])
      if (r) (r as { stopped_at?: string }).stopped_at = '2026-10-24T00:00:00Z'
      return noContent(route)
    }

    // Users / personas
    if (method === 'GET' && path === '/users') {
      return json(route, 200, { items: USER_LIST, total: USER_LIST.length })
    }
    if (method === 'POST' && path === '/users') {
      return route.fulfill({ status: 201, headers: DATE_HEADER() })
    }
    if (method === 'GET' && path === '/personas') {
      return json(route, 200, { items: PERSONAS, total: PERSONAS.length })
    }

    // Telegram link
    if (method === 'GET' && path === '/me/telegram') {
      return json(route, 200, {
        linked: true,
        tg_username: 'e2e',
        dm_enabled: true,
        bot_blocked: false,
        linked_at: '2026-09-01T10:00:00Z',
      })
    }

    // Assistant
    if (method === 'GET' && path === '/assistant/status') {
      return json(route, 200, { ...ASSISTANT_STATUS, enabled: assistantEnabled })
    }
    if (method === 'POST' && path === '/assistant/ask/stream') {
      const frames =
        'event: step\ndata: {"n":1,"tool":"find_occurrences","label":"Ищу платежи по срокам"}\n\n' +
        'event: block\ndata: {"type":"occurrence_list","title":"Просроченные","items":[{"occurrence_id":"' +
        occ[0]!.id +
        '","payment_id":"' +
        occ[0]!.id +
        '","title":"Аренда офиса","amount":"15000","currency":"RUB","display":"15 000 ₽","due_date":"2026-10-19","status":"overdue"}]}\n\n' +
        'event: text\ndata: {"delta":"Просрочен один платёж на 15 000 ₽."}\n\n' +
        'event: done\ndata: {"status":"ok","text":"Просрочен один платёж на 15 000 ₽.","followups":["Покажи все просроченные"],"meta":{"model":"rules"}}\n\n'
      return route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream', ...DATE_HEADER() },
        body: frames,
      })
    }
    if (method === 'POST' && path === '/assistant/ask') {
      return json(route, 200, { status: 'ok', text: 'Ответ ассистента', followups: [] })
    }
    if (method === 'GET' && path === '/assistant/history') {
      return json(route, 200, {
        channel: 'miniapp',
        conversation: 'miniapp',
        items: [],
        total: 0,
        memory: { enabled: true, context_turns: 4, ttl_seconds: 7200 },
      })
    }
    if (method === 'DELETE' && path === '/assistant/history') {
      return noContent(route)
    }

    // Неизвестный маршрут — 500, тест упадёт.
    return text(route, 500, 'unhandled: ' + method + ' ' + path)
  })

  return handle
}
