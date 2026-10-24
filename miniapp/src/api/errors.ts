// Единый тип ошибки API.

export class ApiError extends Error {
  status: number
  code?: string
  requestId?: string
  retryAfter?: number
  network?: boolean

  constructor(status: number, message: string, opts?: Partial<ApiError>) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    if (opts) {
      this.code = opts.code
      this.requestId = opts.requestId
      this.retryAfter = opts.retryAfter
      this.network = opts.network
    }
  }
}

export async function parseError(res: Response): Promise<ApiError> {
  const text = await res.text()
  let message = ''
  let code: string | undefined
  let requestId: string | undefined

  if (text) {
    try {
      const obj = JSON.parse(text) as {
        error?: { code?: string; message?: string; request_id?: string }
      }
      if (obj && obj.error && obj.error.code) {
        code = obj.error.code
        message = obj.error.message || obj.error.code
        requestId = obj.error.request_id
      } else {
        message = text.trim()
      }
    } catch {
      message = text.trim()
    }
  }
  if (!message) message = res.statusText || `HTTP ${res.status}`

  const retryHeader = res.headers.get('Retry-After')
  const retryAfter = retryHeader ? parseInt(retryHeader, 10) : undefined

  return new ApiError(res.status, message, {
    code,
    requestId,
    retryAfter: Number.isNaN(retryAfter) ? undefined : retryAfter,
  })
}
