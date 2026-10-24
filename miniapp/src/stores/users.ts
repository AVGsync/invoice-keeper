import { reactive } from 'vue'
import { ApiError } from '../api/errors'
import { listUsers, createUser, listPersonas } from '../api/endpoints'
import type { CreateUserRequest, Persona, User } from '../api/types'

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
}
