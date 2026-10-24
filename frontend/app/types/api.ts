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
