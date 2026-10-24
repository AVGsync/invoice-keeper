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

// Recurring
export function listRecurring(signal?: AbortSignal): Promise<RecurrentPayment[]> {
  return request('GET', '/recurring-payments', { signal })
}

export function createRecurring(req: CreateRecurringRequest): Promise<{ id: string }> {
  // Без day_offset — он вычисляется из start_date на сервере.
  return request('POST', '/recurring-payments', { body: req })
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

export function listPersonas(signal?: AbortSignal): Promise<PersonaListResponse> {
  return request('GET', '/personas', { signal })
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
