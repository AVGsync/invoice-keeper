import type {
  AddReceiptLinkRequest,
  Receipt,
  ReceiptAccessResponse,
  ReceiptListResponse,
  AssistantAnswerResponse,
  AssistantHistoryResponse,
  AuditActionsResponse,
  AuditLogParams,
  AuditLogResponse,
  PaymentSearchParams,
  PaymentSearchResponse,
  RecurringSearchParams,
  RecurringPaymentSearchResponse,
  AssistantAskRequest,
  AssistantStatusResponse,
  MiniAppAuthRequest,
  MiniAppAuthResponse,
  MiniAppLinkRequest,
  TelegramLoginPollResponse,
  TelegramLoginStartResponse,
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
      // sign-in through the bot: start gives a deep link, poll waits for the confirmation button
      telegramStart: () => api<TelegramLoginStartResponse>('/auth/telegram/start', { method: 'POST' }),
      telegramPoll: (code: string) =>
        api<TelegramLoginPollResponse>('/auth/telegram/poll', { method: 'POST', body: { code } }),
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
      // filtered, sorted and paged by the server
      search: (query: RecurringSearchParams) => api<RecurringPaymentSearchResponse>('/recurring-payments/search', { query }),
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
      // filtered, sorted and paged by the server
      search: (query: PaymentSearchParams) => api<PaymentSearchResponse>('/payment-occurrences/search', { query }),
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
      // a code to the saved address, then the code back: the address is confirmed
      sendCode: () => api('/notifications/email/code', { method: 'POST' }),
      verify: (code: string) => api('/notifications/email/verify', { method: 'POST', body: { code } }),
    },

    // receipts of a payment period: files and links. the assignee and the admin can see and change them
    receipts: {
      list: (periodId: string) => api<ReceiptListResponse>(`/payment-occurrences/${encodeURIComponent(periodId)}/receipts`),
      // multipart, the file goes in the `file` field
      upload: (periodId: string, file: File, title = '') => {
        const body = new FormData()
        body.append('file', file)
        return api<Receipt>(`/payment-occurrences/${encodeURIComponent(periodId)}/receipts`, { method: 'POST', body, query: title ? { title } : undefined })
      },
      addLink: (periodId: string, body: AddReceiptLinkRequest) =>
        api<Receipt>(`/payment-occurrences/${encodeURIComponent(periodId)}/receipts/link`, { method: 'POST', body }),
      // where to open it: the link itself, or a signed address of the file
      access: (id: string) => api<ReceiptAccessResponse>(`/receipts/${encodeURIComponent(id)}/access`),
      remove: (id: string) => api(`/receipts/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },

    // who did what: the whole log for the admin, your own actions for everyone
    audit: {
      list: (query: AuditLogParams) => api<AuditLogResponse>('/audit-log', { query }),
      mine: (query: AuditLogParams) => api<AuditLogResponse>('/me/activity', { query }),
      actions: () => api<AuditActionsResponse>('/audit-log/actions'),
    },

    assistant: {
      status: () => api<AssistantStatusResponse>('/assistant/status'),
      // your earlier questions and answers, kept on the server
      history: (channel = 'web', limit = 20) => api<AssistantHistoryResponse>('/assistant/history', { query: { channel, limit } }),
      clearHistory: (channel = 'web') => api('/assistant/history', { method: 'DELETE', query: { channel } }),
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
