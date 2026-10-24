import { request } from './client'
import { ApiError } from './errors'
import { createSseParser } from './sse'
import { ask as askFallback } from './endpoints'
import type { AssistantBlock, AssistantEnvelope, AssistantStep } from './types'

export interface AskHandlers {
  onStep: (step: AssistantStep) => void
  onBlock: (block: AssistantBlock) => void
  onText: (delta: string) => void
  onDone: (envelope: AssistantEnvelope) => void
  onError: (code: string, message?: string) => void
}

const CODE_MESSAGES: Record<string, string> = {
  llm_unavailable: 'Ассистент временно недоступен',
  timeout: 'Ассистент не успел ответить',
  assistant_timeout: 'Ассистент не успел ответить',
  assistant_busy: 'Ассистент сейчас занят, попробуйте через минуту',
  assistant_disabled: 'Ассистент выключен',
  rate_limited: 'Слишком много запросов, подождите немного',
  data_unavailable: 'Не удалось получить данные',
  bad_question: 'Переформулируйте вопрос',
}

export function codeMessage(code: string): string {
  return CODE_MESSAGES[code] ?? 'Ассистент временно недоступен'
}

interface AskDeps {
  setAssistantDisabled?: () => void
}

// Выполняет вопрос ассистенту: SSE /assistant/ask/stream. Fallback на
// /assistant/ask делаем ТОЛЬКО когда стрим не поддержан (не event-stream,
// сетевая ошибка до заголовков, обрыв без done и без error-кадра). Серверные
// ошибки (error-кадр, 503 busy, 504 timeout, 429) показываем без повторного
// запроса, чтобы не удваивать ожидание и лимиты и не звать LLM дважды.
export async function askAssistant(
  question: string,
  h: AskHandlers,
  signal?: AbortSignal,
  deps: AskDeps = {},
): Promise<void> {
  const streamCtl = new AbortController()
  let internalTimedOut = false
  const streamTimeout = setTimeout(() => {
    internalTimedOut = true
    streamCtl.abort()
  }, 35_000)
  const onAbort = (): void => streamCtl.abort()
  if (signal) {
    if (signal.aborted) streamCtl.abort()
    else signal.addEventListener('abort', onAbort)
  }

  let doneReceived = false
  let errorCode: string | null = null

  const runFallback = async (): Promise<void> => {
    const fbCtl = new AbortController()
    const fbTimeout = setTimeout(() => fbCtl.abort(), 30_000)
    const fbAbort = (): void => fbCtl.abort()
    if (signal) {
      if (signal.aborted) fbCtl.abort()
      else signal.addEventListener('abort', fbAbort)
    }
    try {
      const env = await askFallback(question, fbCtl.signal)
      if (env.status === 'error') {
        h.onError(env.error_code || 'llm_unavailable', env.text)
      } else {
        h.onDone(env)
      }
    } catch (e) {
      if (isAbort(e)) return
      if (e instanceof ApiError && e.code) h.onError(e.code, e.message)
      else h.onError('llm_unavailable')
    } finally {
      clearTimeout(fbTimeout)
      if (signal) signal.removeEventListener('abort', fbAbort)
    }
  }

  try {
    let res: Response
    try {
      res = await request<Response>('POST', '/assistant/ask/stream', {
        body: { question, channel: 'miniapp' },
        raw: true,
        signal: streamCtl.signal,
        headers: { Accept: 'text/event-stream' },
      })
    } catch (e) {
      if (isAbort(e)) {
        // Внутренний таймаут до заголовков — покажем ошибку; пользовательский abort — молча.
        if (!signal?.aborted && internalTimedOut) h.onError('timeout')
        return
      }
      if (e instanceof ApiError) {
        // Сетевая ошибка до заголовков → стрим не поддержан, пробуем обычный /ask.
        if (e.network || e.status === 0) {
          await runFallback()
          return
        }
        // Остальные ошибки до старта потока — без fallback.
        if (e.status === 400) {
          h.onError('bad_question')
          return
        }
        if (e.status === 401) {
          h.onError('llm_unavailable')
          return
        }
        if (e.status === 429) {
          h.onError('rate_limited')
          return
        }
        if (e.status === 503 && e.code === 'assistant_disabled') {
          deps.setAssistantDisabled?.()
          h.onError('assistant_disabled')
          return
        }
        if (e.status === 503) {
          h.onError('assistant_busy')
          return
        }
        if (e.status === 504) {
          h.onError('assistant_timeout')
          return
        }
        h.onError(e.code || 'llm_unavailable')
        return
      }
      await runFallback()
      return
    }

    const contentType = res.headers.get('Content-Type') || ''
    if (!contentType.includes('text/event-stream') || !res.body) {
      await runFallback()
      return
    }

    const reader = res.body.getReader()
    const decoder = new TextDecoder('utf-8')
    const parser = createSseParser((e) => {
      if (e.event === 'step') {
        try {
          h.onStep(JSON.parse(e.data) as AssistantStep)
        } catch {
          /* пропускаем битый кадр */
        }
      } else if (e.event === 'block') {
        try {
          h.onBlock(JSON.parse(e.data) as AssistantBlock)
        } catch {
          /* пропускаем */
        }
      } else if (e.event === 'text') {
        try {
          const obj = JSON.parse(e.data) as { delta?: string }
          if (obj.delta) h.onText(obj.delta)
        } catch {
          /* пропускаем */
        }
      } else if (e.event === 'done') {
        try {
          const env = JSON.parse(e.data) as AssistantEnvelope
          doneReceived = true
          if (env.status === 'error') {
            h.onError(env.error_code || 'llm_unavailable', env.text)
          } else {
            h.onDone(env)
          }
        } catch {
          /* пропускаем */
        }
      } else if (e.event === 'error') {
        try {
          const obj = JSON.parse(e.data) as { code?: string }
          errorCode = obj.code || 'llm_unavailable'
        } catch {
          errorCode = 'llm_unavailable'
        }
      }
    })

    try {
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        parser.push(decoder.decode(value, { stream: true }))
      }
      parser.push(decoder.decode())
      parser.end()
    } catch (e) {
      if (isAbort(e)) {
        // прерывание пользователем или внутренним таймаутом — разбираем ниже
      }
      // сетевой обрыв: если done не пришёл, ниже решим про fallback
    }

    if (!doneReceived) {
      // Пользователь отменил — молча.
      if (signal?.aborted) return
      // Внутренний таймаут — свой код, без повторного запроса.
      if (internalTimedOut) {
        h.onError('timeout')
        return
      }
      // Сервер прислал error-кадр (например, code=timeout) — показываем его, без fallback.
      if (errorCode) {
        h.onError(errorCode)
        return
      }
      // Поток закрылся/оборвался без done и без error-кадра → один fallback на /ask.
      // Уже показанные блоки остаются до ответа fallback.
      await runFallback()
    }
  } finally {
    clearTimeout(streamTimeout)
    if (signal) signal.removeEventListener('abort', onAbort)
  }
}

function isAbort(e: unknown): boolean {
  return (
    (e instanceof DOMException && e.name === 'AbortError') ||
    (e instanceof Error && e.name === 'AbortError')
  )
}
