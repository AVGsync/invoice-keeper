// copied from swagger (docs.json), update when backend changes

export interface LoginRequest {
  login: string
  password: string
}

export interface CreateUserRequest {
  full_name: string // 3-64
  login: string // 3-32, only letters, digits, _ . -
  password: string // 8-72
  persona_id?: string
}

// send at least one field
export interface UserUpdateRequest {
  full_name?: string
  login?: string
}

export interface User {
  id?: string
  full_name?: string
  login?: string
  role?: string
  persona?: PersonaRef | null // null if the persona was deleted
  created_at?: string
  // the user's telegram, from GET /users
  telegram_linked?: boolean
  // email for reminders is saved, and confirmed with the code from the letter
  email_linked?: boolean
  email_verified?: boolean
  telegram_linked_at?: string
  // without the @, '' when not linked or no username
  telegram_username?: string
}

export interface UserListResponse {
  items?: User[]
  total?: number
}

export interface Persona {
  id?: string
  name?: string
  description?: string
  users_count?: number
  created_at?: string
}

export interface PersonaRef {
  id?: string
  name?: string
}

export interface CreatePersonaRequest {
  name?: string // 1-64, must be unique
  description?: string // up to 255
}

// send at least one field, "" clears the description
export interface UpdatePersonaRequest {
  name?: string
  description?: string
}

export interface PersonaListResponse {
  items?: Persona[]
  total?: number
}

// request.AssistantAskRequest — question 1-500 chars, channel defaults to web
export interface AssistantAskRequest {
  question?: string
  channel?: 'web' | 'miniapp'
}

// response.AssistantAnswerResponse — also the `done` frame of the stream
export interface AssistantAnswerResponse {
  answer_id?: string
  // up to 3: occurrence_list, currency_totals, payment_list, people_list
  blocks?: Record<string, unknown>[]
  // when status is error: llm_unavailable | data_unavailable | timeout | bad_question
  error_code?: string
  followups?: string[]
  // model, tokens, cost, latency — admins only
  meta?: Record<string, unknown>
  mode?: 'llm' | 'rules' | 'fake'
  notes?: string[]
  status?: 'ok' | 'partial' | 'error'
  text?: string
}

// response.AssistantStatusResponse — hide the assistant when enabled is false
export interface AssistantStatusResponse {
  enabled?: boolean
  examples?: string[]
  healthy?: boolean
  mode?: string
  memory?: AssistantMemoryInfo
}

// response.AssistantMemoryInfo — how much of the dialogue the assistant keeps in mind
export interface AssistantMemoryInfo {
  enabled?: boolean
  // how many last turns go with the next question
  context_turns?: number
  ttl_seconds?: number
}

// response.AssistantHistoryItem — one question and its short answer
export interface AssistantHistoryItem {
  id?: string
  question?: string
  answer_text?: string
  answer_id?: string
  // ok | partial | error
  status?: string
  error_code?: string | null
  mode?: string
  // goes with the next question as context
  in_context?: boolean
  created_at?: string
}

// response.AssistantHistoryResponse — oldest first
export interface AssistantHistoryResponse {
  channel?: string
  conversation?: string
  items?: AssistantHistoryItem[]
  memory?: AssistantMemoryInfo
  total?: number
}

// response.TelegramLinkResponse
export interface TelegramLinkResponse {
  bot_blocked?: boolean
  dm_enabled?: boolean
  linked?: boolean
  linked_at?: string
  tg_username?: string
}

// response.TelegramLinkCodeResponse
export interface TelegramLinkCodeResponse {
  deep_link?: string
  expires_in?: number
}

// POST /auth/telegram/start
export interface TelegramLoginStartResponse {
  code: string
  deep_link: string
  expires_in: number
}

// POST /auth/telegram/poll: pending while the bot waits, ok once the cookies are set
export interface TelegramLoginPollResponse {
  status: 'pending' | 'ok'
}

// response.TelegramTeamChatResponse
export interface TelegramTeamChatResponse {
  bound?: boolean
  bound_at?: string
  is_active?: boolean
  last_error?: string | null
  tg_chat_id?: number
  title?: string | null
}

// request.MiniAppAuthRequest
export interface MiniAppAuthRequest {
  init_data?: string
}

// request.MiniAppLinkRequest
export interface MiniAppLinkRequest {
  init_data?: string
  login?: string
  password?: string
}

// response.MiniAppUser
export interface MiniAppUser {
  full_name?: string
  id?: string
  role?: 'admin' | 'user'
}

// response.MiniAppAuthResponse — the token stays in memory, no cookies
export interface MiniAppAuthResponse {
  access_token?: string
  dm_available?: boolean
  expires_in?: number
  token_type?: string
  user?: MiniAppUser
}

// ---- payments (Swagger from main, synced 2026-10-24) ----
// amounts are in minor units: kopecks, cents. dates are "YYYY-MM-DD"

export interface RecurrentPayment {
  id?: string
  amount?: number
  currency?: string
  description?: string
  // weekly | monthly
  frequency?: string
  // every N weeks or months
  interval?: number
  // monthly: day of the month 1-31, weekly: day of the week 0-6
  day_offset?: number
  // the first due date, the schedule counts from it
  start_date?: string
  assignee_id?: string | null
  assignee_name?: string
  // set once the payment is stopped
  stopped_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface CreateRecurrentPaymentRequest {
  amount?: number
  currency?: string
  description?: string
  frequency?: string
  interval?: number
  day_offset?: number
  start_date?: string
  assignee_id?: string
}

// the schedule fields can't change, only amount, currency, description and assignee
export interface UpdateRecurrentPaymentRequest {
  amount?: number
  currency?: string
  description?: string
  assignee_id?: string
  frequency?: string
  interval?: number
  day_offset?: number
  start_date?: string
}

export interface PaymentOccurrence {
  id?: string
  // who has to pay it
  user_id?: string
  recurrent_payment_id?: string | null
  amount?: number
  currency?: string
  description?: string
  due_date?: string
  paid_date?: string | null
  paid_late?: boolean
  // pending | overdue | paid | ended
  status?: string
  assignee_name?: string
  // its recurring payment was stopped
  ended?: boolean
  paid_at?: string | null
  created_at?: string
  updated_at?: string
}

// GET /payment-occurrences/search. amounts are in kopecks / cents and need exactly one currency
export interface PaymentSearchParams {
  q?: string
  status?: string[]
  currency?: string[]
  assignee?: 'me'
  assignee_id?: string[]
  persona_id?: string[]
  recurrent_payment_id?: string
  due_from?: string
  due_to?: string
  paid_from?: string
  paid_to?: string
  amount_min?: number
  amount_max?: number
  sort?: 'due_date' | 'amount' | 'created_at' | 'description' | 'paid_date'
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

// response.PaymentSearchResponse — total counts everything that matched, not just this page
export interface PaymentSearchResponse {
  items?: PaymentOccurrence[]
  limit?: number
  offset?: number
  total?: number
}

// GET /recurring-payments/search, the same rules for amounts
export interface RecurringSearchParams {
  q?: string
  currency?: string[]
  frequency?: string[]
  assignee?: 'me'
  assignee_id?: string[]
  persona_id?: string[]
  status?: ('active' | 'stopped')[]
  include_stopped?: boolean
  amount_min?: number
  amount_max?: number
  start_from?: string
  start_to?: string
  next_due_from?: string
  next_due_to?: string
  sort?: 'description' | 'amount' | 'created_at' | 'next_due' | 'start_date'
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

// response.RecurringPaymentItem — a template with its next unpaid due date
export interface RecurringPaymentItem extends RecurrentPayment {
  next_due_date?: string | null
  status?: 'active' | 'stopped'
}

export interface RecurringPaymentSearchResponse {
  items?: RecurringPaymentItem[]
  limit?: number
  offset?: number
  total?: number
}

export interface PaymentListResponse {
  items?: PaymentOccurrence[]
  total?: number
}

// action confirm: mark paid (paid_date defaults to today, on a paid one it changes the date).
// action cancel: take the mark off. no action: admin edits amount / currency / description
export interface PatchPaymentOccurrenceRequest {
  action?: 'confirm' | 'cancel'
  paid_date?: string
  amount?: number
  currency?: string
  description?: string
  due_date?: string
}

export interface PaymentCurrencySummary {
  currency?: string
  paid_total?: number
  outstanding_total?: number
  overdue_total?: number
}

export interface PaymentSummaryResponse {
  items?: PaymentCurrencySummary[]
}

export interface UserNotificationSettings {
  user_id?: string
  email?: string
  email_enabled?: boolean
  email_verified?: boolean
  days_before_due?: number
  // "HH:MM"
  send_time?: string
  timezone?: string
  created_at?: string
  updated_at?: string
}

export interface UpdateUserNotificationSettingsRequest {
  email?: string
  email_enabled?: boolean
  days_before_due?: number
  send_time?: string
  timezone?: string
}

export type PatchUserNotificationSettingsRequest = UpdateUserNotificationSettingsRequest

// response.AuditActor — null when the action was done by the system
export interface AuditActor {
  id?: string
  full_name?: string | null
  role?: string | null
}

// response.AuditLogItem. changes: { field: { from, to } }, only what really changed
export interface AuditLogItem {
  id?: number
  created_at?: string
  actor?: AuditActor | null
  action?: string
  action_label?: string
  // web | miniapp | bot | system
  channel?: string
  // success | failure
  outcome?: string
  entity_type?: string | null
  entity_id?: string | null
  changes?: Record<string, { from?: unknown, to?: unknown }> | null
  payload?: Record<string, unknown> | null
  ip?: string | null
  user_agent?: string | null
  request_id?: string | null
}

export interface AuditLogResponse {
  items?: AuditLogItem[]
  limit?: number
  offset?: number
  total?: number
}

// GET /audit-log (admin) and GET /me/activity (only your own, actor_id is ignored there).
// action can be an exact code or a prefix like `recurring.*`
export interface AuditLogParams {
  actor_id?: string[]
  action?: string[]
  entity_type?: string[]
  entity_id?: string
  channel?: string[]
  outcome?: 'success' | 'failure'
  // YYYY-MM-DD or RFC3339
  from?: string
  to?: string
  q?: string
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

// audit.ActionInfo — every action code with its russian label, for the filters
export interface AuditActionInfo {
  action?: string
  label?: string
  entity_type?: string
}

export interface AuditActionsResponse {
  items?: AuditActionInfo[]
}

// response.ReceiptResponse — a file kept on the server (kind=file) or an outside link (kind=link)
export interface Receipt {
  id?: string
  payment_id?: string
  kind?: 'file' | 'link'
  // '' when not given
  title?: string
  // files only
  file_name?: string
  content_type?: string
  size_bytes?: number
  // links only
  url?: string
  uploaded_by?: string
  uploaded_by_name?: string
  created_at?: string
}

// newest last
export interface ReceiptListResponse {
  items?: Receipt[]
  total?: number
}

// request.AddReceiptLinkRequest — http(s) only, without a scheme https is added
export interface AddReceiptLinkRequest {
  url: string
  title?: string
}

// response.ReceiptAccessResponse — for a file: /api/v1/receipt-files/{id}?exp=…&sig=…,
// opens without logging in until expires_at
export interface ReceiptAccessResponse {
  kind?: 'file' | 'link'
  url?: string
  expires_at?: string
}
