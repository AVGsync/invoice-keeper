// turns a failed request into a message for the screen.
// backend answers either with plain text or with { error: { message } }
export function apiError(e: unknown): string {
  const err = e as { statusCode?: number, data?: unknown }

  if (typeof err.data === 'string' && err.data.trim()) return err.data.trim()

  const message = (err.data as { error?: { message?: string } } | undefined)?.error?.message
  if (message) return message

  return err.statusCode ? `Ошибка ${err.statusCode}` : 'Сервер не отвечает'
}
