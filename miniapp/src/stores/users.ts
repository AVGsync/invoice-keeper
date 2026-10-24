import { reactive } from 'vue'
import { ApiError } from '../api/errors'
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  listPersonas,
  createPersona,
  updatePersona,
  deletePersona,
} from '../api/endpoints'
import type { CreateUserRequest, Persona, UpdateUserRequest, User } from '../api/types'

interface UsersState {
  items: User[]
  personas: Persona[]
  loaded: boolean
  loading: boolean
  personasLoaded: boolean
  error: ApiError | null
}

export const usersState = reactive<UsersState>({
  items: [],
  personas: [],
  loaded: false,
  loading: false,
  personasLoaded: false,
  error: null,
})

export const users = {
  state: usersState,

  async load(): Promise<void> {
    usersState.loading = true
    usersState.error = null
    try {
      const resp = await listUsers()
      usersState.items = resp.items
      usersState.loaded = true
    } catch (e) {
      if (e instanceof ApiError) usersState.error = e
      else throw e
    } finally {
      usersState.loading = false
    }
  },

  async loadPersonas(): Promise<void> {
    try {
      const resp = await listPersonas()
      usersState.personas = resp.items
      usersState.personasLoaded = true
    } catch (e) {
      if (e instanceof ApiError) usersState.error = e
      else throw e
    }
  },

  async create(req: CreateUserRequest): Promise<void> {
    await createUser(req)
    await this.load()
  },

  async update(id: string, req: UpdateUserRequest): Promise<void> {
    await updateUser(id, req)
    await this.load()
  },

  async remove(id: string): Promise<void> {
    await deleteUser(id)
    await this.load()
  },

  async savePersona(id: string | null, name: string, description: string): Promise<void> {
    if (id) await updatePersona(id, { name, description })
    else await createPersona({ name, description: description || undefined })
    await Promise.all([this.loadPersonas(), this.load()])
  },

  async removePersona(id: string): Promise<void> {
    await deletePersona(id)
    await Promise.all([this.loadPersonas(), this.load()])
  },
}
