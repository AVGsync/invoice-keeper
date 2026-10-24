import { ApiError } from '../api/errors'

// Человеко-читаемый текст ошибки на русском.
export function humanError(e: unknown): string {
  if (!(e instanceof ApiError)) {
    return 'Что-то пошло не так'
  }
  if (e.network || e.status === 0) return 'Нет соединения с сервером'
  if (e.status === 403) return 'Недостаточно прав для этого действия'
  if (e.status === 404) return 'Не найдено'
  if (e.status === 413) return 'Файл слишком большой'
  if (e.status === 415) return 'Этот формат не поддерживается: нужен PDF, фото, txt, csv, docx, xlsx'
  if (e.status === 503 && /smtp/i.test(e.message)) return 'Почта на сервере не настроена'
  if (e.status === 429 && /wrong code/i.test(e.message)) return 'Слишком много неверных кодов, запросите новый'
  if (e.status === 429) {
    return e.retryAfter
      ? `Слишком много запросов, подождите ${e.retryAfter} с`
      : 'Слишком много запросов, подождите'
  }
  if (e.status === 502) return 'Не удалось отправить письмо: проверьте настройки почты на сервере'
  if (e.status >= 500) return 'Сервер недоступен, попробуйте позже'

  if (e.status === 400 || e.status === 409) {
    const msg = (e.message || '').toLowerCase()
    if (msg.includes('paid date is in the future')) return 'Дата оплаты не может быть в будущем'
    if (msg.includes('payment already paid')) return 'Платёж уже отмечен оплаченным'
    if (msg.includes('start_date is too far in the past')) return 'Дата не может быть раньше чем год назад'
    if (msg.includes('user with this login already exists')) return 'Этот логин уже занят'
    if (msg.includes('persona not found')) return 'Персона не найдена'
    if (msg.includes('day_offset does not match')) return 'Неверная дата графика'
    if (msg.includes('description must be at most 255')) return 'Название не длиннее 255 символов'
    if (msg.includes('referenced')) return 'Нельзя удалить: на сотрудника ссылаются платежи'
    if (msg.includes('persona with this name')) return 'Персона с таким названием уже есть'
    if (msg.includes('too many receipts')) return 'К платежу уже прикреплено максимум чеков'
    if (msg.includes('url')) return 'Некорректная ссылка: нужен адрес http или https'
    if (msg.includes('invalid verification code')) return 'Неверный код'
    if (msg.includes('expired')) return 'Код устарел или не запрашивался, запросите новый'
    if (msg.includes('already verified')) return 'Почта уже подтверждена'
    if (msg.includes('email')) return 'Проверьте адрес почты'
    if (msg.includes('amount')) return 'Сумма должна быть больше нуля'
    if (msg.includes('login must be')) return 'Логин: 3–32 символа, латиница, цифры, . _ -'
    if (msg.includes('password must be')) return 'Пароль: от 8 символов'
    return `Ошибка в данных: ${e.message}`
  }

  return e.message || 'Что-то пошло не так'
}
