import { reactive } from 'vue'
import { assistantStatus, assistantHistory, clearAssistantHistory } from '../api/endpoints'
import { askAssistant } from '../api/ask'
import { ApiError } from '../api/errors'
import type {
  AssistantBlock,
  AssistantEnvelope,
  AssistantMeta,
  AssistantStatus,
  AssistantStep,
} from '../api/types'

export interface Exchange {
  q: string
  steps: AssistantStep[]
  blocks: AssistantBlock[]
  text: string
  followups: string[]
  status: 'ok' | 'partial' | 'error'
  error?: string
  meta?: AssistantMeta
  busy: boolean
}

interface AssistantState {
  status: AssistantStatus
  history: Exchange[]
}

const MAX_HISTORY = 20

export const assistantState = reactive<AssistantState>({
  status: { enabled: false, healthy: false, mode: '', examples: [] },
  history: [],
})

export const assistant = {
  state: assistantState,
  get status(): AssistantStatus {
    return assistantState.status
  },

  async loadStatus(): Promise<void> {
    const s = await assistantStatus()
    assistantState.status = {
      enabled: !!s.enabled,
      healthy: !!s.healthy,
      mode: s.mode,
      examples: Array.isArray(s.examples) ? s.examples : [],
    }
  },

  // Подтягивает серверную историю диалога в локальный фид, если он пуст.
  async loadHistory(): Promise<void> {
    if (assistantState.history.length) return
    try {
      const res = await assistantHistory(20)
      const restored: Exchange[] = res.items
        .filter((i) => i.in_context)
        .map((i) => ({
          q: i.question,
          steps: [],
          blocks: [],
          text: i.answer_text,
          followups: [],
          status: i.status,
          busy: false,
        }))
      if (restored.length) assistantState.history.push(...restored)
    } catch {
      // молча: истории может не быть, ручка может отсутствовать
    }
  },

  // Начинает новый диалог: просит бэкенд забыть историю и чистит фид.
  async resetConversation(): Promise<void> {
    try {
      await clearAssistantHistory()
    } catch (e) {
      if (e instanceof ApiError && (e.status === 404 || e.status === 405)) {
        assistantState.history.splice(0)
        return
      }
      throw e
    }
    assistantState.history.splice(0)
  },

  async ask(q: string, signal?: AbortSignal): Promise<void> {
    const ex = reactive<Exchange>({
      q,
      steps: [],
      blocks: [],
      text: '',
      followups: [],
      status: 'ok',
      busy: true,
    })
    assistantState.history.push(ex)
    if (assistantState.history.length > MAX_HISTORY) {
      assistantState.history.splice(0, assistantState.history.length - MAX_HISTORY)
    }

    const applyEnvelope = (env: AssistantEnvelope): void => {
      if (env.blocks) ex.blocks = env.blocks
      if (env.text) ex.text = env.text
      if (env.followups) ex.followups = env.followups
      if (env.meta) ex.meta = env.meta
      ex.status = env.status
    }

    await askAssistant(
      q,
      {
        onStep: (s) => ex.steps.push(s),
        onBlock: (b) => ex.blocks.push(b),
        onText: (delta) => {
          ex.text += delta
        },
        onDone: (env) => applyEnvelope(env),
        onError: (code) => {
          ex.status = 'error'
          ex.error = code
          // Текст ошибки берём из codeMessage по коду (в UI), серверное англоязычное сообщение не показываем.
          if (code === 'assistant_disabled') assistantState.status.enabled = false
        },
      },
      signal,
      { setAssistantDisabled: () => (assistantState.status.enabled = false) },
    )
    ex.busy = false
    if (signal?.aborted && ex.status !== 'error') {
      ex.status = 'error'
      ex.error = 'aborted'
    }
  },
}
