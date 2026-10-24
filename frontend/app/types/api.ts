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
