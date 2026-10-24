import { reactive } from 'vue'
import { ApiError } from '../api/errors'
import { listRecurring, createRecurring, stopRecurring, updateRecurring } from '../api/endpoints'
import type { CreateRecurringRequest, RecurrentPayment } from '../api/types'
import { occurrences } from './occurrences'

interface RecState {
  items: RecurrentPayment[]
  loaded: boolean
  loading: boolean
  error: ApiError | null
}

export const recState = reactive<RecState>({
  items: [],
  loaded: false,
  loading: false,
  error: null,
})

export const recurring = {
  state: recState,

  async load(): Promise<void> {
    recState.loading = true
    recState.error = null
    try {
      recState.items = await listRecurring()
      recState.loaded = true
    } catch (e) {
      if (e instanceof ApiError) recState.error = e
      else throw e
    } finally {
      recState.loading = false
    }
  },

  async create(req: CreateRecurringRequest): Promise<{ id: string }> {
    const res = await createRecurring(req)
    await Promise.all([this.load(), occurrences.load({ force: true })])
    return res
  },

  async update(id: string, body: Parameters<typeof updateRecurring>[1]): Promise<void> {
    await updateRecurring(id, body)
    await Promise.all([this.load(), occurrences.load({ force: true })])
  },

  async stop(id: string): Promise<void> {
    await stopRecurring(id)
    await Promise.all([this.load(), occurrences.load({ force: true })])
  },
}
