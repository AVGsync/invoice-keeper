import { reactive, computed } from 'vue'
import { ApiError } from '../api/errors'
import { listOccurrences, getOccurrence } from '../api/endpoints'
import type { PaymentResponse } from '../api/types'
import { session } from './session'

interface OccState {
  items: PaymentResponse[]
  loaded: boolean
  loading: boolean
  error: ApiError | null
  loadedAt: number
}

const STALE_MS = 60_000

export const occState = reactive<OccState>({
  items: [],
  loaded: false,
  loading: false,
  error: null,
  loadedAt: 0,
})

let inflight: Promise<void> | null = null

async function doLoad(): Promise<void> {
  occState.loading = true
  occState.error = null
  try {
    const resp = await listOccurrences()
    occState.items = resp.items
    occState.loaded = true
    occState.loadedAt = Date.now()
  } catch (e) {
    if (e instanceof ApiError) occState.error = e
    else throw e
  } finally {
    occState.loading = false
  }
}

export const occurrences = {
  state: occState,

  load(opts: { force?: boolean } = {}): Promise<void> {
    const fresh = Date.now() - occState.loadedAt < STALE_MS
    if (!opts.force && occState.loaded && fresh) return Promise.resolve()
    if (inflight) return inflight
    inflight = doLoad().finally(() => {
      inflight = null
    })
    return inflight
  },

  markStale(): void {
    occState.loadedAt = 0
  },

  async refreshOne(id: string): Promise<PaymentResponse | null> {
    try {
      const occ = await getOccurrence(id)
      const idx = occState.items.findIndex((i) => i.id === id)
      if (idx >= 0) occState.items.splice(idx, 1, occ)
      else occState.items.push(occ)
      return occ
    } catch (e) {
      if (e instanceof ApiError) throw e
      throw e
    }
  },

  byId(id: string): PaymentResponse | undefined {
    return occState.items.find((i) => i.id === id)
  },

  // История: тот же recurrent_payment_id, кроме самого периода, по убыванию due_date, до 12.
  history(occ: PaymentResponse): PaymentResponse[] {
    if (!occ.recurrent_payment_id) return []
    return occState.items
      .filter(
        (i) => i.recurrent_payment_id === occ.recurrent_payment_id && i.id !== occ.id,
      )
      .sort((a, b) => (a.due_date < b.due_date ? 1 : a.due_date > b.due_date ? -1 : 0))
      .slice(0, 12)
  },
}

export const mine = computed(() =>
  occState.items.filter((i) => session.user && i.user_id === session.user.id),
)
