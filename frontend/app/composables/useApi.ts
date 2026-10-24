import type {
  CreatePersonaRequest,
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
  }
}
