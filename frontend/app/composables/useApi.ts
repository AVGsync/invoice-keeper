import type {
  AssistantAnswerResponse,
  AssistantAskRequest,
  AssistantStatusResponse,
  MiniAppAuthRequest,
  MiniAppAuthResponse,
  MiniAppLinkRequest,
  TelegramLinkCodeResponse,
  TelegramLinkResponse,
  TelegramTeamChatResponse,
  CreatePersonaRequest,
  CreateRecurrentPaymentRequest,
  PatchPaymentOccurrenceRequest,
  PatchUserNotificationSettingsRequest,
  PaymentListResponse,
  PaymentOccurrence,
  PaymentSummaryResponse,
  RecurrentPayment,
  UpdateRecurrentPaymentRequest,
  UpdateUserNotificationSettingsRequest,
  UserNotificationSettings,
  CreateUserRequest,
  LoginRequest,
  Persona,
  PersonaListResponse,
  UpdatePersonaRequest,
  User,
  UserListResponse,
  UserUpdateRequest,
} from '~/types/api'

const api = $fetch.create({ baseURL: '/api/v1' })

export function useApi() {
  return {
    ping: () => api<string>('/ping'),

    auth: {
      login: (body: LoginRequest) => api('/auth/login', { method: 'POST', body }),
      logout: () => api('/auth/logout', { method: 'POST' }),
      me: () => api<User>('/auth/me'),
      refresh: () => api('/auth/refresh', { method: 'POST' }),
    },

    users: {
      list: () => api<UserListResponse>('/users'),
      create: (body: CreateUserRequest) => api('/users', { method: 'POST', body }),
      update: (id: string, body: UserUpdateRequest) =>
        api(`/users/${encodeURIComponent(id)}`, { method: 'PATCH', body }),
      remove: (id: string) => api(`/users/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },

    personas: {
      list: () => api<PersonaListResponse>('/personas'),
      create: (body: CreatePersonaRequest) => api<Persona>('/personas', { method: 'POST', body }),
      update: (id: string, body: UpdatePersonaRequest) =>
        api<Persona>(`/personas/${encodeURIComponent(id)}`, { method: 'PATCH', body }),
      remove: (id: string) => api(`/personas/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },

    // templates: everyone reads, only the admin creates, edits and stops
    recurring: {
      list: () => api<RecurrentPayment[]>('/recurring-payments'),
      create: (body: CreateRecurrentPaymentRequest) =>
        api<{ id?: string }>('/recurring-payments', { method: 'POST', body }),
      update: (id: string, body: UpdateRecurrentPaymentRequest) =>
        api(`/recurring-payments/${encodeURIComponent(id)}`, { method: 'PATCH', body }),
      // stops it: no new periods, the history stays
      stop: (id: string) => api(`/recurring-payments/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },

    // periods of the recurring payments. the assignee or the admin marks them paid
    occurrences: {
      list: (mine = false) => api<PaymentListResponse>('/payment-occurrences', { query: mine ? { assignee: 'me' } : undefined }),
      get: (id: string) => api<PaymentOccurrence>(`/payment-occurrences/${encodeURIComponent(id)}`),
      patch: (id: string, body: PatchPaymentOccurrenceRequest) =>
        api(`/payment-occurrences/${encodeURIComponent(id)}`, { method: 'PATCH', body }),
    },

    // totals per currency, due dates between from and to
    summary: (from?: string, to?: string) => api<PaymentSummaryResponse>('/payments/summary', { query: { from, to } }),

    // the current user's email reminders. 404 means they were never set up
    notifications: {
      get: () => api<UserNotificationSettings>('/notifications/settings'),
      create: (body: UpdateUserNotificationSettingsRequest) => api('/notifications/settings', { method: 'POST', body }),
      replace: (body: UpdateUserNotificationSettingsRequest) => api('/notifications/settings', { method: 'PUT', body }),
      patch: (body: PatchUserNotificationSettingsRequest) => api('/notifications/settings', { method: 'PATCH', body }),
      remove: () => api('/notifications/settings', { method: 'DELETE' }),
    },

    assistant: {
      status: () => api<AssistantStatusResponse>('/assistant/status'),
      ask: (body: AssistantAskRequest) => api<AssistantAnswerResponse>('/assistant/ask', { method: 'POST', body }),
      // server-sent events (step, block, text, done) — read with useAssistant
      askStream: (body: AssistantAskRequest, signal?: AbortSignal) =>
        api<ReadableStream<Uint8Array>>('/assistant/ask/stream', { method: 'POST', body, responseType: 'stream', signal }),
    },

    telegram: {
      // the current user's own Telegram link
      me: () => api<TelegramLinkResponse>('/me/telegram'),
      unlink: () => api('/me/telegram', { method: 'DELETE' }),
      linkCode: () => api<TelegramLinkCodeResponse>('/me/telegram/link-code', { method: 'POST' }),
      // admin: the team chat reminders go to
      teamChat: () => api<TelegramTeamChatResponse>('/telegram/team-chat'),
      teamChatTest: () => api('/telegram/team-chat/test', { method: 'POST' }),
      // Telegram Mini App login. /telegram/webhook is left out: Telegram calls it, not the panel
      miniappAuth: (body: MiniAppAuthRequest) => api<MiniAppAuthResponse>('/telegram/miniapp/auth', { method: 'POST', body }),
      miniappLink: (body: MiniAppLinkRequest) => api<MiniAppAuthResponse>('/telegram/miniapp/link', { method: 'POST', body }),
    },
  }
}
