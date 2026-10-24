import { request } from './client'
import type {
  MiniAppAuthResponse,
  User,
  UserListResponse,
  PersonaListResponse,
  PaymentListResponse,
  PaymentResponse,
  RecurrentPayment,
  CreateRecurringRequest,
  CreateUserRequest,
  TelegramLinkResponse,
  AssistantStatus,
  AssistantEnvelope,
  AssistantHistoryResponse,
  Receipt,
  ReceiptAccess,
  UpdateUserRequest,
  AuditResponse,
  TeamChat,
  Currency,
} from './types'

// Auth
export function miniAppAuth(initData: string): Promise<MiniAppAuthResponse> {
  return request('POST', '/telegram/miniapp/auth', { body: { init_data: initData }, auth: false })
}

export function miniAppLink(
  initData: string,
  login: string,
  password: string,
): Promise<MiniAppAuthResponse> {
  return request('POST', '/telegram/miniapp/link', {
    body: { init_data: initData, login, password },
    auth: false,
  })
}

export function getMe(signal?: AbortSignal): Promise<User> {
  return request('GET', '/auth/me', { signal })
}

// Occurrences
export function listOccurrences(signal?: AbortSignal): Promise<PaymentListResponse> {
  // Всегда без assignee — "мои" вычисляется на клиенте.
  return request('GET', '/payment-occurrences', { signal })
}

export function getOccurrence(id: string, signal?: AbortSignal): Promise<PaymentResponse> {
  return request('GET', `/payment-occurrences/${id}`, { signal })
}

export function confirmOccurrence(id: string, paidDate?: string): Promise<void> {
  const body: Record<string, string> = { action: 'confirm' }
  if (paidDate) body.paid_date = paidDate
  return request('PATCH', `/payment-occurrences/${id}`, { body })
}

export function cancelOccurrence(id: string): Promise<void> {
  return request('PATCH', `/payment-occurrences/${id}`, { body: { action: 'cancel' } })
}

export function editOccurrence(
  id: string,
  body: { amount?: number; currency?: Currency; description?: string },
): Promise<void> {
  return request('PATCH', `/payment-occurrences/${id}`, { body })
}

// Receipts
export async function listReceipts(paymentId: string, signal?: AbortSignal): Promise<Receipt[]> {
  const res = await request<{ items: Receipt[] }>('GET', `/payment-occurrences/${paymentId}/receipts`, { signal })
  return res.items
}

export function uploadReceipt(paymentId: string, file: File, title?: string): Promise<Receipt> {
  const form = new FormData()
  if (title) form.append('title', title)
  form.append('file', file, file.name)
  return request('POST', `/payment-occurrences/${paymentId}/receipts`, { body: form })
}

export function addReceiptLink(paymentId: string, url: string, title?: string): Promise<Receipt> {
  return request('POST', `/payment-occurrences/${paymentId}/receipts/link`, { body: { url, title: title ?? '' } })
}

export function deleteReceipt(id: string): Promise<void> {
  return request('DELETE', `/receipts/${id}`)
}

export function receiptAccess(id: string): Promise<ReceiptAccess> {
  return request('GET', `/receipts/${id}/access`)
}

// Recurring
export function listRecurring(signal?: AbortSignal): Promise<RecurrentPayment[]> {
  return request('GET', '/recurring-payments', { signal })
}

export function createRecurring(req: CreateRecurringRequest): Promise<{ id: string }> {
  // Без day_offset — он вычисляется из start_date на сервере.
  return request('POST', '/recurring-payments', { body: req })
}

export function updateRecurring(
  id: string,
  body: { amount?: number; currency?: Currency; description?: string; assignee_id?: string },
): Promise<void> {
  return request('PATCH', `/recurring-payments/${id}`, { body })
}

export function stopRecurring(id: string): Promise<void> {
  return request('DELETE', `/recurring-payments/${id}`)
}

// Users and personas
export function listUsers(signal?: AbortSignal): Promise<UserListResponse> {
  return request('GET', '/users', { signal })
}

export function createUser(req: CreateUserRequest): Promise<void> {
  return request('POST', '/users', { body: req })
}

export function updateUser(id: string, body: UpdateUserRequest): Promise<void> {
  return request('PATCH', `/users/${id}`, { body })
}

export function deleteUser(id: string): Promise<void> {
  return request('DELETE', `/users/${id}`)
}

export function createPersona(body: { name: string; description?: string }): Promise<unknown> {
  return request('POST', '/personas', { body })
}

export function updatePersona(id: string, body: { name?: string; description?: string }): Promise<unknown> {
  return request('PATCH', `/personas/${id}`, { body })
}

export function deletePersona(id: string): Promise<void> {
  return request('DELETE', `/personas/${id}`)
}

export function listPersonas(signal?: AbortSignal): Promise<PersonaListResponse> {
  return request('GET', '/personas', { signal })
}

// Журнал действий: свои записи или (для админа) все
export function listActivity(
  scope: 'me' | 'all',
  query: { limit: number; offset: number; channel?: string },
  signal?: AbortSignal,
): Promise<AuditResponse> {
  return request('GET', scope === 'me' ? '/me/activity' : '/audit-log', { query, signal })
}

export function getTeamChat(signal?: AbortSignal): Promise<TeamChat> {
  return request('GET', '/telegram/team-chat', { signal })
}

export function testTeamChat(): Promise<void> {
  return request('POST', '/telegram/team-chat/test')
}

// Telegram
export function getTelegramLink(signal?: AbortSignal): Promise<TelegramLinkResponse> {
  return request('GET', '/me/telegram', { signal })
}

// Assistant
export function assistantStatus(signal?: AbortSignal): Promise<AssistantStatus> {
  return request('GET', '/assistant/status', { signal })
}

export function ask(question: string, signal?: AbortSignal): Promise<AssistantEnvelope> {
  return request('POST', '/assistant/ask', {
    body: { question, channel: 'miniapp' },
    signal,
  })
}

export function assistantHistory(
  limit = 20,
  signal?: AbortSignal,
): Promise<AssistantHistoryResponse> {
  return request('GET', '/assistant/history', { query: { channel: 'miniapp', limit }, signal })
}

export function clearAssistantHistory(): Promise<void> {
  return request('DELETE', '/assistant/history', { query: { channel: 'miniapp' } })
}
