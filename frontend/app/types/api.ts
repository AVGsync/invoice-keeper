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
