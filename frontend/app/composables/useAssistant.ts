import type { AssistantAnswerResponse, AssistantStatusResponse } from '~/types/api'

export interface ChatMessage {
  id: number
  from: 'me' | 'assistant'
  text: string
  // the answer isn't here yet: shows the step line instead
  pending?: boolean
  typing?: boolean
  step?: string
  error?: boolean
  followups?: string[]
}

// what the server's error codes mean for the person asking
const errorText: Record<string, string> = {
  assistant_disabled: 'Ассистент выключен.',
  assistant_busy: 'Ассистент занят, спросите чуть позже.',
  assistant_rate_limited: 'Слишком много вопросов. Попробуйте позже.',
  assistant_timeout: 'Ассистент не успел ответить. Попробуйте спросить проще.',
  timeout: 'Ассистент не успел ответить. Попробуйте спросить проще.',
  bad_request: 'Не получилось разобрать вопрос.',
}
const unavailable = 'Ассистент сейчас недоступен. Попробуйте позже или включите демо-режим в настройках.'

// one chat for the whole panel: it stays when you change pages
export function useAssistant() {
  const api = useApi()
  const demo = useDemo()
  const { reduceMotion } = usePrefs()

  const messages = useState<ChatMessage[]>('chat-messages', () => [])
  const open = useState('chat-open', () => false)
  const status = useState<AssistantStatusResponse | null>('chat-status', () => null)
  const nextId = () => (messages.value.at(-1)?.id ?? 0) + 1

  // hidden only when the server says the feature is off (and demo mode can't stand in)
  const available = computed(() => demo.enabled.value || status.value?.enabled !== false)
  const examples = computed(() => (demo.enabled.value ? demoExamples : status.value?.examples ?? []))
  const busy = computed(() => messages.value.some(message => message.pending || message.typing))

  async function loadStatus() {
    if (status.value) return
    status.value = await api.assistant.status().catch(() => ({}))
  }

  let controller: AbortController | undefined

  // the answer appears a few letters at a time, so the window grows with it
  function typeOut(message: ChatMessage, text: string) {
    message.pending = false
    if (reduceMotion.value) {
      message.text = text
      return Promise.resolve()
    }
    message.typing = true
    return new Promise<void>((resolve) => {
      let shown = 0
      const timer = setInterval(() => {
        shown += 3
        message.text = text.slice(0, shown)
        if (shown >= text.length) {
          clearInterval(timer)
          message.typing = false
          resolve()
        }
      }, 16)
    })
  }

  function fail(message: ChatMessage, code?: string) {
    message.pending = false
    message.error = true
    message.text = (code && errorText[code]) || unavailable
  }

  // reads the server-sent events: step, block, text, done, error
  async function readStream(stream: ReadableStream<Uint8Array>, message: ChatMessage) {
    const reader = stream.pipeThrough(new TextDecoderStream() as TransformStream<Uint8Array, string>).getReader()
    let buffer = ''
    let text = ''
    let envelope: AssistantAnswerResponse | undefined

    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += value
      // frames end with an empty line
      let end = buffer.indexOf('\n\n')
      while (end !== -1) {
        const frame = buffer.slice(0, end)
        buffer = buffer.slice(end + 2)
        end = buffer.indexOf('\n\n')

        const event = frame.match(/^event: (.*)$/m)?.[1]
        const data = frame.match(/^data: (.*)$/m)?.[1]
        if (!event || !data) continue
        const payload = JSON.parse(data)
        if (event === 'step') message.step = payload.label
        else if (event === 'text') text += payload.delta ?? ''
        else if (event === 'done') envelope = payload
        else if (event === 'error') return fail(message, payload.code)
      }
    }

    if (!envelope && !text) return fail(message)
    if (envelope?.status === 'error') return fail(message, envelope.error_code)
    await typeOut(message, text || envelope?.text || '')
    message.followups = envelope?.followups
  }

  async function askDemo(question: string, message: ChatMessage) {
    const answer = demoAnswer(question, demo.periods.value)
    message.step = answer.step
    // a short pause, like the real one looking things up
    await new Promise(resolve => setTimeout(resolve, reduceMotion.value ? 0 : 600))
    await typeOut(message, answer.text)
    message.followups = demoExamples.filter(example => example !== question).slice(0, 2)
  }

  async function ask(question: string) {
    question = question.trim()
    if (!question || busy.value) return
    open.value = true
    messages.value.push({ id: nextId(), from: 'me', text: question })
    messages.value.push({ id: nextId(), from: 'assistant', text: '', pending: true, step: 'Думаю' })
    // the reactive copy, so changes below redraw the chat
    const message = messages.value[messages.value.length - 1]!

    if (demo.enabled.value) return askDemo(question, message)

    controller = new AbortController()
    try {
      const stream = await api.assistant.askStream({ question, channel: 'web' }, controller.signal)
      await readStream(stream, message)
    }
    catch (e) {
      if (controller.signal.aborted) return
      const err = e as { data?: { error?: { code?: string } } }
      fail(message, err.data?.error?.code)
    }
  }

  function clear() {
    controller?.abort()
    messages.value = []
  }

  return { messages, open, available, examples, busy, loadStatus, ask, clear }
}
