// Типы ответов API. Формы сверены с backend/docs/swagger.yaml.

export type Currency = 'RUB' | 'USD' | 'EUR'
export type Role = 'admin' | 'user'
export type OccStatus = 'pending' | 'overdue' | 'paid' | 'ended'

export interface MiniUser {
  id: string
  full_name: string
  role: Role
}

export interface MiniAppAuthResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: MiniUser
  dm_available: boolean
}

export interface PersonaRef {
  id: string
  name: string
}

export interface User {
  id: string
  full_name: string
  login: string
  role: Role
  persona: PersonaRef | null
  created_at: string
}

export interface UserListResponse {
  items: User[]
  total: number
}

export interface Persona {
  id: string
  name: string
  description: string | null
  users_count: number
  created_at: string
}

export interface PersonaListResponse {
  items: Persona[]
  total: number
}

export interface PaymentResponse {
  id: string
  user_id: string
  recurrent_payment_id?: string
  amount: number
  currency: Currency
  description: string
  due_date: string
  paid_date?: string
  paid_late?: boolean
  status: OccStatus
  assignee_name?: string
  ended?: boolean
  paid_at?: string
  created_at: string
  updated_at: string
}

export interface PaymentListResponse {
  items: PaymentResponse[]
  total: number
}

export type Frequency = 'weekly' | 'monthly'

export interface RecurrentPayment {
  id: string
  amount: number
  currency: Currency
  description: string
  frequency: Frequency
  interval: number
  day_offset?: number
  start_date: string
  stopped_at?: string
  assignee_id?: string
  assignee_name?: string
  created_at: string
  updated_at: string
}

export interface CreateRecurringRequest {
  description: string
  amount: number
  currency: Currency
  frequency: Frequency
  interval: number
  start_date: string
  assignee_id: string
}

export interface CreateUserRequest {
  full_name: string
  login: string
  password: string
  persona_id?: string
}

export interface TelegramLinkResponse {
  linked: boolean
  tg_username?: string
  dm_enabled?: boolean
  bot_blocked?: boolean
  linked_at?: string
}

export interface AssistantStatus {
  enabled: boolean
  healthy: boolean
  mode?: string
  examples: string[]
}

// Блоки ассистента.
export type AssistantBlockStatus = 'overdue' | 'upcoming' | 'paid'

export interface OccurrenceListItem {
  occurrence_id: string
  payment_id: string
  title: string
  amount: string
  currency: Currency
  display: string
  due_date: string
  status: AssistantBlockStatus
  days_overdue?: number
  assignee_id?: string
  assignee_name?: string
}

export interface CurrencyTotalsEntry {
  total: number | string
  paid: number | string
  unpaid: number | string
  count: number
  display: string
}

export interface PaymentListItem {
  payment_id: string
  title: string
  amount: string
  currency: Currency
  display: string
  assignee_name?: string
  assignee_persona?: string
  next_due?: string
  is_ended: boolean
}

export interface PeopleListItem {
  user_id: string
  full_name: string
  role?: string
  persona?: string
  active_payments?: number
}

export interface OccurrenceListBlock {
  type: 'occurrence_list'
  title?: string
  total?: number
  truncated?: boolean
  items: OccurrenceListItem[]
}

export interface CurrencyTotalsBlock {
  type: 'currency_totals'
  title?: string
  period?: string | null
  coverage_until?: string
  by_currency: Partial<Record<Currency, CurrencyTotalsEntry>>
  groups?: unknown[]
}

export interface PaymentListBlock {
  type: 'payment_list'
  title?: string
  total?: number
  truncated?: boolean
  items: PaymentListItem[]
}

export interface PeopleListBlock {
  type: 'people_list'
  title?: string
  items: PeopleListItem[]
}

export type AssistantBlock =
  | OccurrenceListBlock
  | CurrencyTotalsBlock
  | PaymentListBlock
  | PeopleListBlock
  | { type: string; [k: string]: unknown }

export interface AssistantMeta {
  model?: string
  tokens?: number
  cost_usd?: number
  [k: string]: unknown
}

export interface AssistantEnvelope {
  answer_id?: string
  status: 'ok' | 'partial' | 'error'
  error_code?: string
  mode?: string
  text?: string
  blocks?: AssistantBlock[]
  followups?: string[]
  notes?: string[]
  meta?: AssistantMeta
}

export interface AssistantStep {
  n: number
  tool: string
  label: string
}
