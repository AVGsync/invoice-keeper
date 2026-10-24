// words for the journal: what kind of thing changed, through which channel, and its fields
import type { AuditLogItem } from '~/types/api'

export const entityNames: Record<string, string> = {
  user: 'Пользователь',
  telegram_link: 'Привязка Telegram',
  telegram_chat: 'Командный чат',
  persona: 'Должность',
  recurring_payment: 'Регулярный платёж',
  occurrence: 'Период',
  receipt: 'Чек',
  notification_settings: 'Напоминания на почту',
  assistant_turn: 'Вопрос ассистенту',
  assistant_conversation: 'Диалог с ассистентом',
}

export const channelNames: Record<string, string> = {
  web: 'Панель',
  miniapp: 'Mini App',
  bot: 'Бот',
  system: 'Система',
}

const fieldNames: Record<string, string> = {
  amount: 'Сумма',
  currency: 'Валюта',
  description: 'Название',
  due_date: 'Срок',
  paid_date: 'Дата оплаты',
  paid_late: 'С опозданием',
  start_date: 'Первый срок',
  frequency: 'Периодичность',
  interval: 'Интервал',
  assignee_id: 'Ответственный',
  login: 'Логин',
  full_name: 'Имя',
  name: 'Название',
  email: 'Почта',
  email_enabled: 'Письма',
  days_before_due: 'Дней до срока',
  send_time: 'Время отправки',
  timezone: 'Часовой пояс',
  reason: 'Причина',
  actor_id: 'Кто',
  // what the assistant entries carry
  mode: 'Как отвечал',
  status: 'Ответ',
  error_code: 'Причина',
  channel: 'Откуда',
  conversation: 'Диалог',
  deleted: 'Удалено сообщений',
  // telegram
  via: 'Через',
  tg_username: 'Telegram',
  expires_in: 'Ссылка действует',
  title: 'Чат',
  type: 'Тип чата',
  answer: 'Ответ',
  // receipts
  kind: 'Чек',
  file_name: 'Файл',
  size_bytes: 'Размер',
  host: 'Сайт',
  // the rest
  to_email: 'Кому',
  password_changed: 'Пароль изменён',
  already_stopped: 'Уже был прекращён',
  noop: 'Без изменений',
}

// words for the values the server writes as codes
const valueNames: Record<string, Record<string, string>> = {
  mode: { llm: 'нейросеть', rules: 'по правилам', fake: 'тестовый режим' },
  status: { ok: 'полный', partial: 'неполный', error: 'не получился', active: 'действует', stopped: 'прекращён' },
  channel: { web: 'Панель', miniapp: 'Mini App', bot: 'Бот', system: 'Система', all: 'везде' },
  via: { web: 'Панель', miniapp: 'Mini App', bot: 'Бот' },
  kind: { file: 'файл', link: 'ссылка' },
  type: { group: 'группа', supergroup: 'группа', private: 'личный', channel: 'канал' },
  reason: {
    invalid_credentials: 'неверный логин или пароль',
    invalid_init_data: 'данные Telegram не прошли проверку',
    init_data_expired: 'данные Telegram устарели',
    not_linked: 'Telegram не привязан',
    already_linked: 'Telegram уже привязан к другому аккаунту',
    no_team_chat: 'командный чат не привязан',
    send_failed: 'сообщение не отправилось',
    invalid_token: 'сессия недействительна',
    no_token: 'нет сессии',
    token_error: 'сессию не удалось проверить',
    user_not_found: 'пользователь не найден',
  },
  error_code: {
    rate_limited: 'слишком много вопросов',
    busy: 'ассистент был занят',
    bad_question: 'вопрос не подошёл',
    unavailable: 'ассистент недоступен',
    timeout: 'не успел ответить',
    llm_unavailable: 'нейросеть недоступна',
    data_unavailable: 'данные недоступны',
  },
}

// ids and file types mean nothing to a person, the entry already says what it's about
const hiddenKeys = new Set(['persona_id', 'occurrence_id', 'previous_chat_id', 'content_type'])

// what goes first, so the one line in the row is the most telling one
const firstKeys = ['name', 'description', 'full_name', 'reason', 'error_code', 'login', 'file_name', 'host', 'status', 'mode', 'amount']

// the server still says «персона» in a few labels, the panel calls it «должность»
export function actionLabel(label = '', action = ''): string {
  if (action.startsWith('persona.')) return label.replace(/персона/i, 'должность')
  return label || action
}

export const fieldName = (key: string) => fieldNames[key] ?? key

// one value as a person reads it: kopecks as rubles, dates as dd.mm.yyyy, yes/no
export function fieldValue(key: string, value: unknown, currency?: string): string {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'boolean') return value ? 'да' : 'нет'
  const named = valueNames[key]?.[String(value)]
  if (named) return named
  if (key === 'conversation') return conversationName(String(value))
  if (key === 'tg_username') return `@${value}`
  if (key === 'size_bytes' && typeof value === 'number') return fileSize(value)
  if (key === 'expires_in' && typeof value === 'number') return `${Math.round(value / 60)} мин.`
  if (key === 'amount' && typeof value === 'number') return currency ? money(value, currency) : formatAmount(value / 100)
  if (key === 'frequency') return value === 'weekly' ? 'еженедельно' : value === 'monthly' ? 'ежемесячно' : String(value)
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}/.test(value)) return toRu(value.slice(0, 10))
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

// the assistant keeps one dialog per place: the panel, the mini app, telegram
function conversationName(value: string): string {
  if (value === 'tg:dm') return 'личные сообщения в Telegram'
  if (value.startsWith('tg:')) return 'групповой чат в Telegram'
  return valueNames.channel![value] ?? value
}

function fileSize(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} КБ`
  return `${(bytes / 1024 / 1024).toFixed(1).replace('.', ',')} МБ`
}

// the currency of the entry, so amounts can be shown with their sign
export function currencyOf(item: AuditLogItem): string | undefined {
  const fromChanges = item.changes?.currency?.to
  const fromPayload = item.payload?.currency
  const value = typeof fromChanges === 'string' ? fromChanges : fromPayload
  return typeof value === 'string' ? value : undefined
}

// «Сумма: 1 000 ₽ → 1 500 ₽», for the row and the details
export function changeLines(item: AuditLogItem): string[] {
  const currency = currencyOf(item)
  return Object.entries(item.changes ?? {}).map(([key, change]) =>
    `${fieldName(key)}: ${fieldValue(key, change?.from, currency)} → ${fieldValue(key, change?.to, currency)}`)
}

// what the entry was about, from its payload: «Название: Аренда офиса», «Сумма: 1 500 ₽»
export function payloadLines(item: AuditLogItem): string[] {
  const currency = currencyOf(item)
  const rank = (key: string) => (firstKeys.includes(key) ? firstKeys.indexOf(key) : firstKeys.length)
  return Object.entries(item.payload ?? {})
    .filter(([key]) => !hiddenKeys.has(key))
    // the currency is already next to the amount
    .filter(([key]) => key !== 'currency' || !item.payload?.amount)
    // «откуда» is already its own column
    .filter(([key, value]) => key !== 'channel' || value !== item.channel)
    .sort(([a], [b]) => rank(a) - rank(b))
    .map(([key, value]) => `${labelOf(item, key)}: ${fieldValue(key, value, currency)}`)
}

// a position has a name and a description, a payment's description is its name
const labelOf = (item: AuditLogItem, key: string) => (item.entity_type === 'persona' && key === 'description' ? 'Описание' : fieldName(key))
