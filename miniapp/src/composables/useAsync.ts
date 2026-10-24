import { ref, shallowRef, type Ref } from 'vue'
import { ApiError } from '../api/errors'

export interface AsyncState<T> {
  data: Ref<T | null>
  loading: Ref<boolean>
  error: Ref<ApiError | null>
  run: () => Promise<void>
}

// Для загрузок уровня экрана, не являющихся сторами (например /me/telegram).
export function useAsync<T>(fn: (signal: AbortSignal) => Promise<T>): AsyncState<T> {
  const data = shallowRef<T | null>(null)
  const loading = ref(false)
  const error = ref<ApiError | null>(null)
  let controller: AbortController | null = null

  const run = async (): Promise<void> => {
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = null
    try {
      data.value = await fn(controller.signal)
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') return
      if (e instanceof ApiError) error.value = e
      else error.value = new ApiError(0, 'unknown')
    } finally {
      loading.value = false
    }
  }

  return { data, loading, error, run }
}
