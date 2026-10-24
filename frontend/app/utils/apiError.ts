// turns a failed request into a message for the screen.
// backend answers either with plain text or with { error: { code, message } }, both in english.
// we look the code or the text up in the lists below and show it in russian.
// when the backend itself is down, the proxy in front of it (nginx) answers with an html page:
// that one is never shown as is, the person gets a plain sentence instead
export function apiError(e: unknown): string {
  const err = e as { statusCode?: number, data?: unknown }
  const status = err.statusCode

  const body = err.data as { error?: { code?: string, message?: string } } | string | undefined
  const code = typeof body === 'object' ? body?.error?.code : undefined
  const message = (typeof body === 'string' ? body : body?.error?.message ?? '').trim()

  if (code && byCode[code]) return byCode[code]

  const known = fromMessage(message)
  if (known) return known

  // already in russian and short: the server said it for people, show it
  if (/[а-яё]/i.test(message) && !/<[a-z!/]/i.test(message) && message.length <= 200) return message

  return statusText(status)
}

// the server's codes (the json answers)
const byCode: Record<string, string> = {
  invalid_credentials: 'Неверный логин или пароль',
  unauthorized: 'Нужно войти заново',
  forbidden: 'Недостаточно прав',
  rate_limit_exceeded: 'Слишком много попыток, попробуйте позже',
  too_many_link_codes: 'Слишком много попыток, попробуйте позже',
  assistant_rate_limited: 'Слишком много вопросов, попробуйте позже',
  assistant_busy: 'Ассистент сейчас занят, спросите чуть позже',
  assistant_disabled: 'Ассистент выключен',
  assistant_unavailable: 'Ассистент сейчас недоступен',
  assistant_timeout: 'Ассистент не успел ответить, попробуйте ещё раз',
  telegram_disabled: 'Бот сейчас выключен',
  telegram_not_linked: 'Telegram не привязан',
  telegram_already_linked: 'Этот Telegram уже привязан к другому аккаунту',
  team_chat_not_bound: 'Командный чат не привязан',
  telegram_error: 'Telegram не принял сообщение',
  init_data_expired: 'Данные Telegram устарели, откройте приложение заново',
  invalid_init_data: 'Данные Telegram не прошли проверку',
}

// the server's plain texts, and the reasons inside «invalid payment: …»
const byMessage: Record<string, string> = {
  // login and session
  'invalid credentials': 'Неверный логин или пароль',
  'invalid login or password': 'Неверный логин или пароль',
  'unauthorized': 'Нужно войти заново',
  'no refresh token': 'Нужно войти заново',
  'invalid refresh token': 'Нужно войти заново',
  'failed get claims': 'Нужно войти заново',
  'forbidden': 'Недостаточно прав',
  'too many requests': 'Слишком много попыток, попробуйте позже',
  'too many questions, slow down': 'Слишком много вопросов, попробуйте позже',

  // users
  'user with this login already exists': 'Такой логин уже занят',
  'login already taken': 'Такой логин уже занят',
  'login is required': 'Введите логин',
  'login must be 3-32 chars: letters, digits, . _ -': 'Логин: от 3 до 32 символов, латинские буквы, цифры, точка, _ и -',
  'full_name must be 3-64 characters': 'Имя: от 3 до 64 символов',
  'password is required': 'Введите пароль',
  'password must be 8-72 bytes': 'Пароль слишком короткий или слишком длинный',
  'password must be at least 8 characters': 'Пароль слишком короткий',
  'password must be at most 72 bytes': 'Пароль слишком длинный',
  'user not found': 'Пользователь не найден',
  'user is referenced and cannot be deleted': 'Пользователя нельзя удалить: на нём есть платежи',
  'admin account is managed via .env': 'Администратора меняют в настройках сервера',
  'persona_id must be a uuid': 'Выберите должность из списка',
  'persona_id must be a valid uuid or empty': 'Выберите должность из списка',
  'at least one field must be provided': 'Ничего не изменилось',

  // positions
  'persona not found': 'Должность не найдена',
  'persona with this name already exists': 'Такая должность уже есть',
  'name must be 1-64 characters': 'Название: от 1 до 64 символов',

  // payments
  'payment not found': 'Платёж не найден',
  'recurrent payment not found': 'Регулярный платёж не найден',
  'payment already paid': 'Этот платёж уже оплачен',
  'payment ended': 'Этот платёж уже прекращён',
  'payment action forbidden': 'Это не ваш платёж',
  'only admin can edit a payment': 'Менять платёж может только администратор',
  'due_date cannot be changed': 'Срок поменять нельзя',
  'paid payment cannot be edited': 'Оплаченный платёж менять нельзя',
  'paid date is in the future': 'Дата оплаты не может быть в будущем',
  'paid_date must be yyyy-mm-dd': 'Проверьте дату оплаты',
  'action confirm is required to set paid_date': 'Дату оплаты можно указать только вместе с отметкой об оплате',
  'amount must be greater than zero': 'Сумма должна быть больше нуля',
  'currency must be one of rub, usd, eur': 'Валюта: RUB, USD или EUR',
  'description is required': 'Введите название',
  'description must be 1..255 characters': 'Название слишком длинное',
  'description must be at most 255 characters': 'Название слишком длинное',
  'frequency must be weekly or monthly': 'Периодичность: еженедельно или ежемесячно',
  'interval must be greater than zero': 'Интервал должен быть больше нуля',
  'assignee not found': 'Ответственный не найден',
  'assignee_id must be a valid uuid': 'Выберите ответственного из списка',
  'start_date must be yyyy-mm-dd': 'Проверьте дату первого срока',
  'start_date is too far in the past': 'Первый срок слишком далеко в прошлом',
  'day_offset does not match start_date': 'День платежа не совпадает с первым сроком',
  'schedule is immutable (day_offset)': 'Расписание платежа поменять нельзя',
  'schedule is immutable (frequency)': 'Периодичность поменять нельзя',
  'schedule is immutable (interval)': 'Интервал поменять нельзя',
  'schedule is immutable (start_date)': 'Первый срок поменять нельзя',

  // search and filters
  'amount_min must be <= amount_max': 'Сумма «от» больше суммы «до»',
  'amount_min/amount_max require exactly one currency': 'Для фильтра по сумме выберите одну валюту',
  'due_from must be <= due_to': 'Начало периода позже конца',
  'paid_from must be <= paid_to': 'Начало периода позже конца',
  'next_due_from must be <= next_due_to': 'Начало периода позже конца',
  'start_from must be <= start_to': 'Начало периода позже конца',
  'from must be <= to': 'Начало периода позже конца',
  'from must be yyyy-mm-dd': 'Проверьте дату',
  'to must be yyyy-mm-dd': 'Проверьте дату',
  'q must be at most 100 characters': 'Слишком длинный запрос',

  // receipts
  'file is too large': 'Файл слишком большой',
  'receipt file too large': 'Файл слишком большой',
  'unsupported file type: allowed pdf, images, txt, csv, docx, xlsx, odt, ods': 'Такой файл не подходит: нужен PDF, картинка, TXT, CSV или документ Office',
  'receipt file type is not supported': 'Такой файл не подходит: нужен PDF, картинка, TXT, CSV или документ Office',
  'too many receipts for this payment': 'У этого платежа уже максимум чеков',
  'receipt not found': 'Чек не найден',
  'field file is required': 'Выберите файл',
  'file is empty': 'Файл пустой',
  'multipart/form-data expected': 'Не получилось отправить файл, попробуйте ещё раз',
  'url is required': 'Вставьте ссылку',
  'url is not valid': 'Ссылка не похожа на адрес сайта',
  'url is too long': 'Ссылка слишком длинная',
  'url must not contain credentials': 'Уберите логин и пароль из ссылки',
  'only http and https links are allowed': 'Нужна ссылка, начинающаяся с http или https',

  // notifications and mail
  'email is required': 'Введите почту',
  'email must not be empty': 'Введите почту',
  'email must be a valid email address': 'Проверьте адрес почты',
  'email must be a valid email address of at most 320 characters': 'Проверьте адрес почты',
  'to must be a plain email address': 'Проверьте адрес почты',
  'days_before_due must be between 0 and 365': 'Дней до срока: от 0 до 365',
  'send_time must be in hh:mm format': 'Время в виде ЧЧ:ММ',
  'timezone is required': 'Выберите часовой пояс',
  'timezone must be a valid iana timezone': 'Выберите часовой пояс из списка',
  'notification settings already exist': 'Напоминания уже настроены',
  'notification settings not found': 'Напоминания ещё не настроены',
  'user notification settings not found': 'Напоминания ещё не настроены',
  'notification settings not found: save the email first': 'Сначала сохраните почту',
  'email already verified': 'Почта уже подтверждена',
  'invalid verification code': 'Неверный код',
  'verification code expired or not requested': 'Код устарел, запросите новый',
  'too many wrong codes, request a new one': 'Слишком много неверных кодов, запросите новый',
  'verification code was sent recently': 'Код уже отправлен, новый можно запросить чуть позже',

  // telegram
  'telegram bot is disabled': 'Бот сейчас выключен',
  'telegram account is not linked': 'Telegram не привязан',
  'telegram account is already linked to another user': 'Этот Telegram уже привязан к другому аккаунту',
  'telegram is already linked to another account': 'Этот Telegram уже привязан к другому аккаунту',
  'too many active link codes': 'Слишком много попыток, попробуйте позже',
  'link code is invalid or expired': 'Ссылка устарела, получите новую',
  'team chat is not bound': 'Командный чат не привязан',
  'no active team chat': 'Командный чат не привязан',
  'failed to send test message': 'Telegram не принял сообщение',

  // assistant
  'question must be 1-500 characters without control symbols': 'Вопрос слишком длинный или пустой',
  'invalid question': 'Не получилось понять вопрос',
  'assistant is busy, try again shortly': 'Ассистент сейчас занят, спросите чуть позже',
  'assistant is disabled': 'Ассистент выключен',
  'assistant is unavailable': 'Ассистент сейчас недоступен',
  'assistant did not answer in time': 'Ассистент не успел ответить, попробуйте ещё раз',

  // anything else the request itself got wrong
  'invalid request body': 'Проверьте заполненные поля',
  'not found': 'Не найдено',
}

function fromMessage(message: string): string | undefined {
  const text = message.toLowerCase()
  if (!text) return undefined
  if (byMessage[text]) return byMessage[text]
  // «invalid payment: amount must be greater than zero» → the part after the colon
  const reason = text.replace(/^invalid (recurrent payment|payment|receipt): /, '')
  if (byMessage[reason]) return byMessage[reason]
  // the mail server is not set up on the backend
  if (text.startsWith('smtp is not configured')) return 'Почта на сервере не настроена'
  // the mail server didn't take the letter, its own words follow after the colon
  if (text.startsWith('failed to send email')) return 'Не получилось отправить письмо, проверьте адрес'
  return undefined
}

// what the status alone tells us, in words
function statusText(status?: number): string {
  if (!status) return 'Сервер не отвечает. Проверьте интернет и попробуйте ещё раз'
  if (status === 502 || status === 503 || status === 504) return 'Сервер временно недоступен. Попробуйте чуть позже'
  if (status >= 500) return 'Ошибка на сервере. Попробуйте ещё раз'
  if (status === 429) return 'Слишком много попыток, попробуйте позже'
  if (status === 413) return 'Файл слишком большой'
  if (status === 409) return 'Не получилось: данные уже изменились, обновите страницу'
  if (status === 404) return 'Не найдено'
  if (status === 403) return 'Недостаточно прав'
  if (status === 401) return 'Нужно войти заново'
  if (status === 400) return 'Проверьте заполненные поля'
  return `Ошибка ${status}`
}
