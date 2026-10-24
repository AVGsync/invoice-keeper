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
  // from an earlier visit (GET /assistant/history) and no longer remembered for the next question
  forgotten?: boolean
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
const unavailable = 'Ассистент сейчас недоступен. Попробуйте позже.'

// the answer being streamed right now, one for the whole panel
let controller: AbortController | undefined

// stops an answer that's still coming in (logout uses it too)
export function stopAssistant() {
  controller?.abort()
  controller = undefined
}

// one chat for the whole panel: it stays when you change pages
export function useAssistant() {
  const api = useApi()
  const { reduceMotion } = usePrefs()

  const messages = useState<ChatMessage[]>('chat-messages', () => [])
  const open = useState('chat-open', () => false)
  const status = useState<AssistantStatusResponse | null>('chat-status', () => null)
  // the earlier dialogue is read from the server once, when the chat is first opened
  const historyLoaded = useState('chat-history-loaded', () => false)
  const nextId = () => (messages.value.at(-1)?.id ?? 0) + 1

  // hidden only when the server says the feature is off
  const available = computed(() => status.value?.enabled !== false)
  const examples = computed(() => status.value?.examples ?? [])
  const busy = computed(() => messages.value.some(message => message.pending || message.typing))

  async function loadStatus() {
    if (status.value) return
    status.value = await api.assistant.status().catch(() => ({}))
  }

  // «помнит 4 последних вопроса», shown in the chat header
  const memory = computed(() => {
    const info = status.value?.memory
    if (!info?.enabled || !info.context_turns) return ''
    const n = info.context_turns
    return `помнит ${n} ${plural(n, ['последний вопрос', 'последних вопроса', 'последних вопросов'])}`
  })

  async function loadHistory() {
    if (historyLoaded.value) return
    historyLoaded.value = true
    const history = await api.assistant.history('web').catch(() => null)
    // something was asked while it loaded: that chat is newer, leave it
    if (!history?.items?.length || messages.value.length) return
    let id = 0
    messages.value = history.items.flatMap((item) => {
      const failed = item.status === 'error'
      return [
        { id: ++id, from: 'me' as const, text: item.question ?? '', forgotten: !item.in_context },
        {
          id: ++id,
          from: 'assistant' as const,
          text: failed ? (item.error_code && errorText[item.error_code]) || unavailable : item.answer_text ?? '',
          error: failed,
          forgotten: !item.in_context,
        },
      ]
    })
  }

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

  async function ask(question: string) {
    question = question.trim()
    if (!question || busy.value) return
    open.value = true
    messages.value.push({ id: nextId(), from: 'me', text: question })
    messages.value.push({ id: nextId(), from: 'assistant', text: '', pending: true, step: 'Думаю' })
    // the reactive copy, so changes below redraw the chat
    const message = messages.value[messages.value.length - 1]!

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

  // a new chat: the server forgets the old one too, so it doesn't go with the next question.
  // if that request fails, the old dialogue shows up again next time and can be cleared then
  function clear() {
    stopAssistant()
    messages.value = []
    api.assistant.clearHistory('web').catch(() => {})
  }

  return { messages, open, available, examples, memory, busy, loadStatus, loadHistory, ask, clear }
}
