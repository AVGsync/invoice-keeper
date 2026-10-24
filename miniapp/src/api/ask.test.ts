import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { askAssistant, type AskHandlers } from './ask'
import { setToken, registerReauth } from './client'
import type { AssistantBlock, AssistantEnvelope, AssistantStep } from './types'

function sseResponse(frames: string[]): Response {
  const body = frames.join('')
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(body))
      controller.close()
    },
  })
  return new Response(stream, {
    status: 200,
    headers: { 'Content-Type': 'text/event-stream' },
  })
}

function jsonErr(status: number, code: string): Response {
  return new Response(JSON.stringify({ error: { code, message: code } }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function envRes(env: AssistantEnvelope): Response {
  return new Response(JSON.stringify(env), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function handlers(): {
  h: AskHandlers
  steps: AssistantStep[]
  blocks: AssistantBlock[]
  texts: string[]
  dones: AssistantEnvelope[]
  errors: { code: string; msg?: string }[]
} {
  const steps: AssistantStep[] = []
  const blocks: AssistantBlock[] = []
  const texts: string[] = []
  const dones: AssistantEnvelope[] = []
  const errors: { code: string; msg?: string }[] = []
  const h: AskHandlers = {
    onStep: (s) => steps.push(s),
    onBlock: (b) => blocks.push(b),
    onText: (t) => texts.push(t),
    onDone: (d) => dones.push(d),
    onError: (code, msg) => errors.push({ code, msg }),
  }
  return { h, steps, blocks, texts, dones, errors }
}

describe('askAssistant', () => {
  beforeEach(() => {
    setToken('TOK')
    registerReauth(async () => {})
  })
  afterEach(() => vi.restoreAllMocks())

  it('нормальный поток step → block → text → done, без fallback', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask')) askCalls.push(url)
      return Promise.resolve(
        sseResponse([
          'event: step\ndata: {"n":1,"tool":"find_occurrences","label":"Ищу"}\n\n',
          'event: block\ndata: {"type":"people_list","items":[]}\n\n',
          'event: text\ndata: {"delta":"Готово"}\n\n',
          'event: done\ndata: {"status":"ok","text":"Готово"}\n\n',
        ]),
      )
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, steps, blocks, texts, dones, errors } = handlers()
    await askAssistant('вопрос', h)
    expect(steps).toHaveLength(1)
    expect(blocks).toHaveLength(1)
    expect(texts).toEqual(['Готово'])
    expect(dones).toHaveLength(1)
    expect(errors).toHaveLength(0)
    expect(askCalls).toHaveLength(0)
  })

  it('503 assistant_busy до потока → без fallback, код assistant_busy', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask/stream')) return Promise.resolve(jsonErr(503, 'assistant_busy'))
      if (url.endsWith('/assistant/ask')) {
        askCalls.push(url)
        return Promise.resolve(envRes({ status: 'ok', text: 'fb' }))
      }
      return Promise.reject(new Error('unexpected'))
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, dones, errors } = handlers()
    await askAssistant('q', h)
    expect(askCalls).toHaveLength(0)
    expect(dones).toHaveLength(0)
    expect(errors[0]?.code).toBe('assistant_busy')
  })

  it('504 assistant_timeout до потока → без fallback, код assistant_timeout', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask/stream')) return Promise.resolve(jsonErr(504, 'assistant_timeout'))
      if (url.endsWith('/assistant/ask')) {
        askCalls.push(url)
        return Promise.resolve(envRes({ status: 'ok', text: 'fb' }))
      }
      return Promise.reject(new Error('unexpected'))
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, errors } = handlers()
    await askAssistant('q', h)
    expect(askCalls).toHaveLength(0)
    expect(errors[0]?.code).toBe('assistant_timeout')
  })

  it('503 assistant_disabled → без fallback, enabled=false', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask/stream'))
        return Promise.resolve(jsonErr(503, 'assistant_disabled'))
      if (url.endsWith('/assistant/ask')) {
        askCalls.push(url)
        return Promise.resolve(envRes({ status: 'ok' }))
      }
      return Promise.reject(new Error('unexpected'))
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, errors } = handlers()
    let disabled = false
    await askAssistant('q', h, undefined, { setAssistantDisabled: () => (disabled = true) })
    expect(disabled).toBe(true)
    expect(askCalls).toHaveLength(0)
    expect(errors[0]?.code).toBe('assistant_disabled')
  })

  it('поток без done → ровно один /ask', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask/stream'))
        return Promise.resolve(sseResponse(['event: step\ndata: {"n":1,"tool":"x","label":"l"}\n\n']))
      if (url.endsWith('/assistant/ask')) {
        askCalls.push(url)
        return Promise.resolve(envRes({ status: 'ok', text: 'fb' }))
      }
      return Promise.reject(new Error('unexpected'))
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, steps, dones } = handlers()
    await askAssistant('q', h)
    expect(steps).toHaveLength(1)
    expect(askCalls).toHaveLength(1)
    expect(dones[0]?.text).toBe('fb')
  })

  it('кадр error → без fallback, показываем код из кадра', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask/stream'))
        return Promise.resolve(
          sseResponse(['event: error\ndata: {"code":"timeout"}\n\n']),
        )
      if (url.endsWith('/assistant/ask')) {
        askCalls.push(url)
        return Promise.resolve(envRes({ status: 'ok', text: 'fb' }))
      }
      return Promise.reject(new Error('unexpected'))
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, dones, errors } = handlers()
    await askAssistant('q', h)
    expect(askCalls).toHaveLength(0)
    expect(dones).toHaveLength(0)
    expect(errors[0]?.code).toBe('timeout')
  })

  it('сетевая ошибка (TypeError) → fallback', async () => {
    const askCalls: string[] = []
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/assistant/ask/stream')) return Promise.reject(new TypeError('failed'))
      if (url.endsWith('/assistant/ask')) {
        askCalls.push(url)
        return Promise.resolve(envRes({ status: 'ok', text: 'fb' }))
      }
      return Promise.reject(new Error('unexpected'))
    })
    vi.stubGlobal('fetch', fetchMock)
    const { h, dones } = handlers()
    await askAssistant('q', h)
    expect(askCalls).toHaveLength(1)
    expect(dones[0]?.text).toBe('fb')
  })
})
